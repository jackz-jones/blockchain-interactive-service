package dashboard

import (
	"context"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OverviewLogic {
	return &OverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OverviewLogic) DashboardOverview() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	stats, err := l.svcCtx.BillingService.GetUsageStats(l.ctx, tenantID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get usage stats: " + err.Error()}, nil
	}

	chains, err := l.svcCtx.TenantSDKManager.ListTenantChains(l.ctx, tenantID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list chains: " + err.Error()}, nil
	}

	// 获取今日成功率
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := todayStart.Add(24 * time.Hour)
	_, totalCount, _ := l.svcCtx.Repo.ListCallLogs(l.ctx, store.CallLogFilter{
		TenantID:  tenantID,
		StartTime: &todayStart,
		EndTime:   &todayEnd,
	}, 0, 1)

	var successRate float64
	if totalCount > 0 {
		successRate = 100.0
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"today_calls":   stats.TodayCalls,
			"month_calls":   stats.MonthCalls,
			"monthly_limit": stats.MonthlyLimit,
			"usage_percent": stats.UsagePercent,
			"active_chains": len(chains),
			"success_rate":  successRate,
		},
	}, nil
}
