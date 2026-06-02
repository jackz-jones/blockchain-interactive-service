package event

import (
	"context"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	commonEvent "github.com/jackz-jones/common/event"

	"github.com/zeromicro/go-zero/core/logx"
)

type PollLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPollLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PollLogic {
	return &PollLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PollLogic) EventPoll(req *types.EventPollRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	if req.SubscriptionId == "" {
		return &types.CommonResponse{Code: 400, Message: "subscription_id is required"}, nil
	}

	// 查找订阅信息
	subVal, ok := subscriptionStore.Load(req.SubscriptionId)
	if !ok {
		return &types.CommonResponse{Code: 404, Message: "subscription not found"}, nil
	}
	subInfo := subVal.(*SubscriptionInfo)

	// 校验租户归属
	if subInfo.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "subscription not owned by current tenant"}, nil
	}

	// 从 Redis stream 读取事件
	var events []commonEvent.CrossChainEvent
	err = l.svcCtx.RedisClient.SubscribeCrossChainEventFromStream(
		l.ctx,
		subInfo.ChainConfigID,
		subInfo.ContractConfigID,
		subInfo.GroupName,
		subInfo.ConsumerName,
		func(event commonEvent.CrossChainEvent) error {
			events = append(events, event)
			return nil
		},
		false,                // wantTrimOldMsg
		10,                   // ackCountThreshold
		100*time.Millisecond, // block（短阻塞，快速返回）
	)
	if err != nil {
		l.Logger.Errorf("poll events error (may be no stream yet): %v", err)
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"subscription_id": req.SubscriptionId,
			"events":          events,
			"count":           len(events),
		},
	}, nil
}
