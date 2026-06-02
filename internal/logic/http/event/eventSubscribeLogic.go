package event

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// subscriptionStore 存储订阅信息（内存管理）
var (
	subscriptionStore   = sync.Map{} // subscriptionID -> *SubscriptionInfo
	subscriptionCounter uint64
	subscriptionMu      sync.Mutex
)

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	ID               string `json:"id"`
	TenantID         uint   `json:"tenant_id"`
	ChainConfigID    uint   `json:"chain_config_id"`
	ContractConfigID uint   `json:"contract_config_id"`
	ChainName        string `json:"chain_name"`
	ContractName     string `json:"contract_name"`
	GroupName        string `json:"group_name"`
	ConsumerName     string `json:"consumer_name"`
	CreatedAt        string `json:"created_at"`
}

type SubscribeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubscribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscribeLogic {
	return &SubscribeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscribeLogic) EventSubscribe(req *types.EventSubscribeRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	if req.ChainName == "" {
		return &types.CommonResponse{Code: 400, Message: "chain_name is required"}, nil
	}
	if req.ContractName == "" && req.ContractAddr == "" {
		return &types.CommonResponse{Code: 400, Message: "contract_name or contract_addr is required"}, nil
	}

	// 通过映射层定位到 DB 记录
	result, err := l.svcCtx.ConfigResolver.Resolve(l.ctx, tenantID, req.ChainName, req.ContractName, req.ContractAddr)
	if err != nil {
		return &types.CommonResponse{Code: 404, Message: err.Error()}, nil
	}

	// 生成订阅 ID
	subscriptionMu.Lock()
	subscriptionCounter++
	subID := fmt.Sprintf("sub_%d_%d_%d", tenantID, result.ContractConfigID, subscriptionCounter)
	subscriptionMu.Unlock()

	// 创建消费者组信息
	groupName := fmt.Sprintf("group_%s", subID)
	consumerName := fmt.Sprintf("consumer_%s", subID)

	// 存储订阅信息
	subInfo := &SubscriptionInfo{
		ID:               subID,
		TenantID:         tenantID,
		ChainConfigID:    result.ChainConfigID,
		ContractConfigID: result.ContractConfigID,
		ChainName:        result.ChainName,
		ContractName:     result.ContractName,
		GroupName:        groupName,
		ConsumerName:     consumerName,
		CreatedAt:        time.Now().Format("2006-01-02T15:04:05Z07:00"),
	}
	subscriptionStore.Store(subID, subInfo)

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"subscription_id":    subID,
			"chain_config_id":    result.ChainConfigID,
			"contract_config_id": result.ContractConfigID,
			"chain_name":         result.ChainName,
			"contract_name":      result.ContractName,
		},
	}, nil
}
