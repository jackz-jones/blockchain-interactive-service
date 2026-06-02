package dashboard

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUsageStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUsageStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsageStatsLogic {
	return &GetUsageStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUsageStatsLogic) GetUsageStats() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	stats, err := l.svcCtx.BillingService.GetUsageStats(l.ctx, tenantID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get usage stats: " + err.Error()}, nil
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: stats}, nil
}
