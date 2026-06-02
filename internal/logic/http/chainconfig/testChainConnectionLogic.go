package chainconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestChainConnectionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestChainConnectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestChainConnectionLogic {
	return &TestChainConnectionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestChainConnectionLogic) TestChainConnection(req *types.ChainConfigIdPathRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required"}, nil
	}

	chainConfig, err := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.Id)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get chain config: " + err.Error()}, nil
	}
	if chainConfig == nil {
		return &types.CommonResponse{Code: 404, Message: "chain config not found"}, nil
	}
	if chainConfig.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "chain config not owned by current tenant"}, nil
	}

	_, testErr := l.svcCtx.TenantSDKManager.GetTenantSDKClient(l.ctx, tenantID, chainConfig.ChainName)

	result := map[string]interface{}{
		"chain_name": chainConfig.ChainName,
		"chain_type": chainConfig.ChainType,
		"success":    testErr == nil,
	}
	if testErr != nil {
		result["error"] = testErr.Error()
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: result}, nil
}
