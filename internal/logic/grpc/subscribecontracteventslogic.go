package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// streamConnectionLimiter 流连接资源管理器
// 限制单个租户的最大并发流连接数，防止资源耗尽
var streamConnectionLimiter = &StreamLimiter{
	maxPerTenant: 10, // 默认每租户最大 10 个并发流连接
}

// StreamLimiter 流连接限制器
type StreamLimiter struct {
	maxPerTenant int64
	tenantCounts sync.Map // tenantID -> *int64 (atomic counter)
}

// Acquire 尝试获取一个流连接槽位，成功返回 true
func (l *StreamLimiter) Acquire(tenantID uint) bool {
	counterVal, _ := l.tenantCounts.LoadOrStore(tenantID, new(int64))
	counter := counterVal.(*int64)
	current := atomic.AddInt64(counter, 1)
	if current > l.maxPerTenant {
		atomic.AddInt64(counter, -1)
		return false
	}
	return true
}

// Release 释放一个流连接槽位
func (l *StreamLimiter) Release(tenantID uint) {
	if counterVal, ok := l.tenantCounts.Load(tenantID); ok {
		counter := counterVal.(*int64)
		atomic.AddInt64(counter, -1)
	}
}

type SubscribeContractEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubscribeContractEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscribeContractEventsLogic {
	return &SubscribeContractEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscribeContractEventsLogic) SubscribeContractEvents(
	in *pb.SubscribeContractEventsRequest,
	stream pb.ChainInteractive_SubscribeContractEventsServer,
) error {
	// 从 context 中获取租户 ID（由 Stream 拦截器注入）
	tenantID := middleware.GetTenantID(stream.Context())
	if tenantID == 0 {
		return status.Error(codes.Unauthenticated, "unauthorized")
	}

	// 流连接资源限制检查
	if !streamConnectionLimiter.Acquire(tenantID) {
		return status.Errorf(codes.ResourceExhausted,
			"exceeded maximum concurrent stream connections (limit: %d)", streamConnectionLimiter.maxPerTenant)
	}
	defer streamConnectionLimiter.Release(tenantID)

	// 参数校验
	if in.ChainName == "" {
		return status.Error(codes.InvalidArgument, "chain_name is required")
	}
	if in.ContractName == "" && in.ContractAddr == "" {
		return status.Error(codes.InvalidArgument, "contract_name or contract_addr is required")
	}

	// 通过配置解析器定位合约配置
	result, err := l.svcCtx.ConfigResolver.Resolve(stream.Context(), tenantID, in.ChainName, in.ContractName, in.ContractAddr)
	if err != nil {
		return status.Errorf(codes.NotFound, "contract not found: %v", err)
	}

	// 校验合约是否已开启事件订阅
	contract, err := l.svcCtx.Repo.GetContractConfig(stream.Context(), result.ContractConfigID)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get contract config: %v", err)
	}
	if contract == nil || !contract.EnableSubscribe {
		return status.Error(codes.FailedPrecondition,
			"该合约未开启事件订阅，请先在合约配置或订阅管理中开启")
	}

	// 生成消费者组和消费者名称
	groupName := fmt.Sprintf("grpc_stream_%d_%d_%d", tenantID, result.ChainConfigID, result.ContractConfigID)
	consumerName := fmt.Sprintf("consumer_%d_%d", tenantID, time.Now().UnixNano())

	l.Logger.Infof("gRPC stream subscription started: tenant=%d, chain=%s, contract=%s, group=%s",
		tenantID, in.ChainName, result.ContractName, groupName)

	// 持续从 Redis Stream 读取事件并推送
	ctx := stream.Context()
	blockDuration := 3 * time.Second // 阻塞等待时间

	for {
		select {
		case <-ctx.Done():
			l.Logger.Infof("gRPC stream subscription ended (client disconnected): tenant=%d, group=%s",
				tenantID, groupName)
			return nil
		default:
		}

		// 从 Redis Stream 读取事件
		err := l.svcCtx.RedisClient.SubscribeCrossChainEventFromStream(
			ctx,
			result.ChainConfigID,
			result.ContractConfigID,
			groupName,
			consumerName,
			func(event commonEvent.CrossChainEvent) error {
				// 将事件数据转为 JSON 字符串
				dataStr := string(event.EventData)
				if !json.Valid(event.EventData) {
					dataStr = fmt.Sprintf("%q", event.EventData)
				}

				// 通过 gRPC 流推送事件
				resp := &pb.ContractEventResponse{
					EventName:       event.EventName,
					ContractAddress: contract.ContractAddr,
					TxHash:          "", // CrossChainEvent 中无此字段，留空
					BlockNumber:     0,  // CrossChainEvent 中无此字段，留空
					Data:            dataStr,
					Timestamp:       time.Now().Unix(),
				}

				if err := stream.Send(resp); err != nil {
					return fmt.Errorf("stream send error: %w", err)
				}
				return nil
			},
			false,         // wantTrimOldMsg
			10,            // ackCountThreshold
			blockDuration, // block
		)

		if err != nil {
			// 如果是 context 取消（客户端断开），正常退出
			if ctx.Err() != nil {
				l.Logger.Infof("gRPC stream subscription ended (context cancelled): tenant=%d, group=%s",
					tenantID, groupName)
				return nil
			}
			// Redis Stream 不存在等情况，等待后重试
			l.Logger.Debugf("stream read returned (may be no stream yet): %v, retrying...", err)
			time.Sleep(1 * time.Second)
		}
	}
}
