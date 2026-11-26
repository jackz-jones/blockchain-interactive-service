package contractconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListContractConfigsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListContractConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContractConfigsLogic {
	return &ListContractConfigsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListContractConfigsLogic) ListContractConfigs(
	req *types.ListContractConfigsRequest,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	if !validateChainConfigOwnership(l.ctx, l.svcCtx, tenantID, req.ChainConfigId) {
		return &types.CommonResponse{Code: 403, Message: "chain config not found or not owned by current tenant"}, nil
	}

	configs, err := l.svcCtx.Repo.ListContractConfigsByChain(l.ctx, req.ChainConfigId)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list contract configs: " + err.Error()}, nil
	}

	// 关联查询链名称并附加到返回数据中
	chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.ChainConfigId)
	chainName := ""
	if chainConfig != nil {
		chainName = chainConfig.ChainName
	}

	items := make([]map[string]interface{}, 0, len(configs))
	for _, cfg := range configs {
		item := map[string]interface{}{
			"ID":               cfg.ID,
			"CreatedAt":        cfg.CreatedAt,
			"UpdatedAt":        cfg.UpdatedAt,
			"DeletedAt":        cfg.DeletedAt,
			"tenant_id":        cfg.TenantID,
			"chain_config_id":  cfg.ChainConfigID,
			"contract_name":    cfg.ContractName,
			"contract_addr":    cfg.ContractAddr,
			"abi_json":         cfg.AbiJSON,
			"enable_subscribe": cfg.EnableSubscribe,
			"extra_conf":       cfg.ExtraConf,
			"chain_name":       chainName,
		}
		items = append(items, item)
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total": len(configs),
			"items": items,
		},
	}, nil
}

// validateChainConfigOwnership 校验链配置归属
func validateChainConfigOwnership(ctx context.Context, svcCtx *svc.ServiceContext, tenantID, chainConfigID uint) bool {
	chainConfigs, err := svcCtx.Repo.ListChainConfigsByTenant(ctx, tenantID)
	if err != nil {
		return false
	}
	for _, cc := range chainConfigs {
		if cc.ID == chainConfigID {
			return true
		}
	}
	return false
}
