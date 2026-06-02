package contractconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContractConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetContractConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContractConfigLogic {
	return &GetContractConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetContractConfigLogic) GetContractConfig(
	req *types.GetContractConfigRequest,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	config, err := l.svcCtx.Repo.GetContractConfig(l.ctx, req.Id)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get contract config: " + err.Error()}, nil
	}
	if config == nil {
		return &types.CommonResponse{Code: 404, Message: "contract config not found"}, nil
	}
	if config.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "contract config not owned by current tenant"}, nil
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: config}, nil
}
