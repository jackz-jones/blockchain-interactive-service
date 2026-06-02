package contractconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteContractConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteContractConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteContractConfigLogic {
	return &DeleteContractConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteContractConfigLogic) DeleteContractConfig(
	req *types.DeleteContractConfigRequest,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required for write operations"}, nil
	}

	existing, err := l.svcCtx.Repo.GetContractConfig(l.ctx, req.Id)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get contract config: " + err.Error()}, nil
	}
	if existing == nil {
		return &types.CommonResponse{Code: 404, Message: "contract config not found"}, nil
	}
	if existing.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "contract config not owned by current tenant"}, nil
	}

	if err := l.svcCtx.Repo.DeleteContractConfig(l.ctx, req.Id); err != nil {
		return &types.CommonResponse{Code: 500, Message: "delete contract config: " + err.Error()}, nil
	}

	chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.ChainConfigId)
	if chainConfig != nil {
		l.svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, chainConfig.ChainName)
	}

	return &types.CommonResponse{Code: 0, Message: "success"}, nil
}
