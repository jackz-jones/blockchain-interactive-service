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

type GetUsageStatsTrendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	R      *http.Request
}

func NewGetUsageStatsTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsageStatsTrendLogic {
	return &GetUsageStatsTrendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUsageStatsTrendLogic) GetUsageStatsTrend() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	// 解析 period 参数，默认 30 天
	days := 30
	if periodStr := l.R.URL.Query().Get("period"); periodStr != "" {
		switch periodStr {
		case "7d":
			days = 7
		case "30d":
			days = 30
		case "90d":
			days = 90
		default:
			if d, err := strconv.Atoi(periodStr); err == nil && d > 0 && d <= 365 {
				days = d
			}
		}
	}

	trend, err := l.svcCtx.BillingService.GetUsageStatsTrend(l.ctx, tenantID, days)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get usage stats trend: " + err.Error()}, nil
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: trend}, nil
}
