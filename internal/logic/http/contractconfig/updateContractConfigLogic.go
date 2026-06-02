package contractconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateContractConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateContractConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateContractConfigLogic {
	return &UpdateContractConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateContractConfigLogic) UpdateContractConfig(
	req *types.UpdateContractConfigRequest,
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

	if req.ContractName == "" {
		return &types.CommonResponse{Code: 400, Message: "contract_name is required"}, nil
	}

	existing.ContractName = req.ContractName
	existing.ContractAddr = req.ContractAddr
	existing.AbiJSON = req.AbiJson
	existing.ExtraConf = req.ExtraConf

	if err := l.svcCtx.Repo.UpdateContractConfig(l.ctx, existing); err != nil {
		return &types.CommonResponse{Code: 500, Message: "update contract config: " + err.Error()}, nil
	}

	chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.ChainConfigId)
	if chainConfig != nil {
		l.svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, chainConfig.ChainName)
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: existing}, nil
}
