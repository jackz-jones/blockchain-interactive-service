package tenant

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DisableTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDisableTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableTenantLogic {
	return &DisableTenantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DisableTenantLogic) DisableTenant(req *types.TenantIdPathRequest) (resp *types.CommonResponse, err error) {
	if err := l.svcCtx.TenantService.DisableTenant(l.ctx, req.Id); err != nil {
		return &types.CommonResponse{Code: 500, Message: "disable tenant: " + err.Error()}, nil
	}
	return &types.CommonResponse{Code: 0, Message: "success"}, nil
}
