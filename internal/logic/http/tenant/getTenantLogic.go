package tenant

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTenantLogic {
	return &GetTenantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTenantLogic) GetTenant(req *types.TenantIdPathRequest) (resp *types.CommonResponse, err error) {
	t, err := l.svcCtx.TenantService.GetTenant(l.ctx, req.Id)
	if err != nil {
		if err == tenant.ErrTenantNotFound {
			return &types.CommonResponse{Code: 404, Message: "tenant not found"}, nil
		}
		return &types.CommonResponse{Code: 500, Message: "get tenant: " + err.Error()}, nil
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: t}, nil
}
