package dashboard

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateBillsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGenerateBillsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateBillsLogic {
	return &GenerateBillsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GenerateBillsLogic) GenerateBills(req *types.GenerateBillsRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	switch req.Type {
	case "daily":
		if err := l.svcCtx.BillingService.GenerateDailyBills(l.ctx); err != nil {
			return &types.CommonResponse{Code: 500, Message: "generate daily bills: " + err.Error()}, nil
		}
	case "monthly":
		if err := l.svcCtx.BillingService.GenerateMonthlyBills(l.ctx); err != nil {
			return &types.CommonResponse{Code: 500, Message: "generate monthly bills: " + err.Error()}, nil
		}
	default:
		return &types.CommonResponse{Code: 400, Message: "invalid type, must be daily or monthly"}, nil
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
	}, nil
}
