package dashboard

import (
	"context"
	"net/http"
	"strconv"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const ()

type GetUsageStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	R      *http.Request
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
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	// 解析 period 参数，默认 30 天
	days := 30
	if periodStr := l.R.URL.Query().Get("period"); periodStr != "" {
		switch periodStr {
		case s7d:
			days = 7
		case s30d:
			days = 30
		case s90d:
			days = 90
		default:
			// 尝试解析为数字
			if d, err := strconv.Atoi(periodStr); err == nil && d > 0 && d <= 365 {
				days = d
			}
		}
	}

	trend, err := l.svcCtx.BillingService.GetUsageStatsTrend(l.ctx, tenantID, days)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get usage stats: " + err.Error()}, nil
	}

	return &types.CommonResponse{Code: 0, Message: successStr, Data: trend}, nil
}
