package event

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnsubscribeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnsubscribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnsubscribeLogic {
	return &UnsubscribeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnsubscribeLogic) EventUnsubscribe(
	req *types.EventUnsubscribeRequest,
) (resp *types.CommonResponse, err error) {
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

	// 清理订阅
	subscriptionStore.Delete(req.SubscriptionId)

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"message":         "subscription deleted",
			"subscription_id": req.SubscriptionId,
		},
	}, nil
}
