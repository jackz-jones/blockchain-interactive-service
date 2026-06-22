package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

type RecentEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRecentEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecentEventsLogic {
	return &RecentEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// RecentEventsRequest 查看最新事件请求
type RecentEventsRequest struct {
	ContractConfigID string `path:"contractConfigId"`
}

// GetRecentEvents 获取指定合约的最新10条事件数据
func (l *RecentEventsLogic) GetRecentEvents(contractConfigIdStr string) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	// 解析合约配置ID
	contractConfigID, err := strconv.ParseUint(contractConfigIdStr, 10, 64)
	if err != nil {
		return &types.CommonResponse{Code: 400, Message: "invalid contract_config_id"}, nil
	}

	// 查询合约配置，确认属于当前租户且已开启订阅
	contract, err := l.svcCtx.Repo.GetContractConfig(l.ctx, uint(contractConfigID))
	if err != nil {
		l.Logger.Errorf("failed to get contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: "internal error"}, nil
	}
	if contract == nil {
		return &types.CommonResponse{Code: 404, Message: "contract config not found"}, nil
	}

	// 校验租户归属（直接使用合约配置上的 TenantID 字段）
	if contract.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "forbidden"}, nil
	}

	// 校验是否开启了订阅
	if !contract.EnableSubscribe {
		return &types.CommonResponse{Code: 400, Message: "该合约未开启事件订阅"}, nil
	}

	// 加载关联的链配置（用于返回链名称）
	chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, contract.ChainConfigID)
	chainName := ""
	if chainConfig != nil {
		chainName = chainConfig.ChainName
	}

	// 构建 Redis Stream key: CrossChainEvent#<chainConfigID>#<contractConfigID>
	streamKey := fmt.Sprintf("%s#%d#%d", commonEvent.CrossChainEventPrefix, contract.ChainConfigID, contract.ID)

	// 使用 XREVRANGE 从 Stream 中倒序读取最新 10 条消息
	messages, err := l.svcCtx.RedisClient.RedisClient.XRevRangeN(l.ctx, streamKey, "+", "-", 10).Result()
	if err != nil {
		l.Logger.Errorf("failed to XREVRANGE from stream %s: %v", streamKey, err)
		// Stream 不存在时返回空列表而非报错
		return &types.CommonResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"items": []interface{}{},
				"total": 0,
			},
		}, nil
	}

	// 解析事件数据
	var items []map[string]interface{}
	for _, msg := range messages {
		eventData, ok := msg.Values[commonEvent.RedisEventKey]
		if !ok {
			continue
		}

		eventStr, ok := eventData.(string)
		if !ok {
			continue
		}

		// 解析 CrossChainEvent 结构
		var crossEvent commonEvent.CrossChainEvent
		if err := json.Unmarshal([]byte(eventStr), &crossEvent); err != nil {
			l.Logger.Debugf("failed to unmarshal event: %v", err)
			continue
		}

		// 解析内部事件数据为可读格式
		var eventDataParsed interface{}
		if json.Valid(crossEvent.EventData) {
			_ = json.Unmarshal(crossEvent.EventData, &eventDataParsed)
		} else {
			eventDataParsed = string(crossEvent.EventData)
		}

		item := map[string]interface{}{
			"message_id": msg.ID,
			"event_name": crossEvent.EventName,
			"data":       eventDataParsed,
		}
		items = append(items, item)
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"items":         items,
			"total":         len(items),
			"chain_name":    chainName,
			"contract_name": contract.ContractName,
			"contract_addr": contract.ContractAddr,
		},
	}, nil
}
