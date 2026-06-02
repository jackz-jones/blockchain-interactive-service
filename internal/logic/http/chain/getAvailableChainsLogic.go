package chain

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAvailableChainsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAvailableChainsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAvailableChainsLogic {
	return &GetAvailableChainsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAvailableChainsLogic) GetAvailableChains() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	// 从数据库加载租户链配置
	dbConfigs, err := l.svcCtx.Repo.ListChainConfigsByTenant(l.ctx, tenantID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list chain configs: " + err.Error()}, nil
	}

	// 收集结果
	chains := make([]map[string]interface{}, 0)
	for _, dbConf := range dbConfigs {
		if !dbConf.Enable {
			continue
		}

		// 获取合约配置
		contractConfigs, _ := l.svcCtx.Repo.ListContractConfigsByChain(l.ctx, dbConf.ID)
		contractNames := make([]string, 0)
		for _, cc := range contractConfigs {
			contractNames = append(contractNames, cc.ContractName)
		}

		chainInfo := map[string]interface{}{
			"chain_name":    dbConf.ChainName,
			"chain_type":    dbConf.ChainType,
			"enable":        dbConf.Enable,
			"contracts":     contractNames,
			"client_active": false,
			"config_id":     dbConf.ID,
		}

		// 检查客户端活跃状态
		status := l.svcCtx.TenantSDKManager.GetClientStatus(tenantID, dbConf.ChainName)
		if active, ok := status["client_active"].(bool); ok {
			chainInfo["client_active"] = active
		}

		chains = append(chains, chainInfo)
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"chains": chains,
			"total":  len(chains),
		},
	}, nil
}