package tenant

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnableTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEnableTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableTenantLogic {
	return &EnableTenantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EnableTenantLogic) EnableTenant(req *types.TenantIdPathRequest) (resp *types.CommonResponse, err error) {
	if err := l.svcCtx.TenantService.EnableTenant(l.ctx, req.Id); err != nil {
		return &types.CommonResponse{Code: 500, Message: "enable tenant: " + err.Error()}, nil
	}
	return &types.CommonResponse{Code: 0, Message: "success"}, nil
}
