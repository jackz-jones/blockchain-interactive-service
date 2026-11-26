package dashboard

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRealtimeCostLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRealtimeCostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRealtimeCostLogic {
	return &GetRealtimeCostLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRealtimeCostLogic) GetRealtimeCost() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	// 获取租户信息（含 Plan 套餐）
	t, err := l.svcCtx.Repo.GetTenantByID(l.ctx, tenantID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get tenant: " + err.Error()}, nil
	}
	if t == nil {
		return &types.CommonResponse{Code: 404, Message: "tenant not found"}, nil
	}

	plan := t.Plan
	if plan == "" {
		plan = "free"
	}

	// 计算实时费用（与定时任务使用同一 calculateAmount，确保一致性）
	cost, err := l.svcCtx.BillingService.GetRealtimeCost(l.ctx, tenantID, plan)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "calculate realtime cost: " + err.Error()}, nil
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: cost}, nil
}
