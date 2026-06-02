package chainconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/jackz-jones/blockchain-interactive-service/internal/validator"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateChainConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateChainConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateChainConfigLogic {
	return &UpdateChainConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateChainConfigLogic) UpdateChainConfig(req *types.UpdateChainConfigRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required for write operations"}, nil
	}

	if req.ChainType != "" {
		if err := validator.ValidateChainType(req.ChainType); err != nil {
			return &types.CommonResponse{Code: 400, Message: err.Error()}, nil
		}
		oldReq := toOldChainConfigRequest(&req.CreateChainConfigRequest)
		if err := validator.ValidateChainConfig(oldReq); err != nil {
			return &types.CommonResponse{Code: 400, Message: err.Error()}, nil
		}
	}

	before, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.Id)
	if before != nil && before.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "chain config not owned by current tenant"}, nil
	}

	config := buildChainConfigModel(tenantID, &req.CreateChainConfigRequest)
	config.ID = req.Id

	if err := l.svcCtx.Repo.UpdateChainConfig(l.ctx, config); err != nil {
		return &types.CommonResponse{Code: 500, Message: "update chain config: " + err.Error()}, nil
	}

	if req.Nodes != nil {
		nodes := buildChainNodesModel(config.ID, req.Nodes)
		if err := l.svcCtx.Repo.ReplaceChainNodes(l.ctx, config.ID, nodes); err != nil {
			return &types.CommonResponse{Code: 500, Message: "replace chain nodes: " + err.Error()}, nil
		}
	}

	l.svcCtx.TenantSDKManager.InvalidateTenantCacheByID(config.ID)

	return &types.CommonResponse{Code: 0, Message: "success", Data: config}, nil
}
