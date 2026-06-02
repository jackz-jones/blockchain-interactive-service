package chainconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/jackz-jones/blockchain-interactive-service/internal/validator"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetChainConfigDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetChainConfigDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetChainConfigDetailLogic {
	return &GetChainConfigDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetChainConfigDetailLogic) GetChainConfigDetail(req *types.ChainConfigIdPathRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
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

	nodes, _ := l.svcCtx.Repo.ListChainNodesByConfigID(l.ctx, chainConfig.ID)
	contractConfigs, _ := l.svcCtx.Repo.ListContractConfigsByChain(l.ctx, chainConfig.ID)
	clientStatus := l.svcCtx.TenantSDKManager.GetClientStatus(tenantID, chainConfig.ChainName)
	maskedConfig := validator.MaskChainConfigSensitiveFields(chainConfig)

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"config":        maskedConfig,
			"nodes":         nodes,
			"contracts":     contractConfigs,
			"client_status": clientStatus,
		},
	}, nil
}
