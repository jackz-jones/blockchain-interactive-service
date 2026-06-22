package chainconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteChainConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteChainConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteChainConfigLogic {
	return &DeleteChainConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteChainConfigLogic) DeleteChainConfig(req *types.ChainConfigIdPathRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required for write operations"}, nil
	}

	before, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.Id)
	if before != nil && before.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "chain config not owned by current tenant"}, nil
	}

	if err := l.svcCtx.Repo.DeleteChainConfig(l.ctx, req.Id); err != nil {
		return &types.CommonResponse{Code: 500, Message: "delete chain config: " + err.Error()}, nil
	}

	if before != nil {
		l.svcCtx.TenantSDKManager.StopAllSubscriptions(tenantID, before.ChainName, before.ID)
	} else {
		l.svcCtx.TenantSDKManager.InvalidateAllTenantCache(tenantID)
	}

	return &types.CommonResponse{Code: 0, Message: "success"}, nil
}
