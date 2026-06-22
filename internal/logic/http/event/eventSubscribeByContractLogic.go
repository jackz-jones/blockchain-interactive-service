package event

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubscribeByContractLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubscribeByContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscribeByContractLogic {
	return &SubscribeByContractLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscribeByContractLogic) SubscribeByContract(req *types.SubscribeByContractRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	if req.ContractConfigID == 0 {
		return &types.CommonResponse{Code: 400, Message: "contract_config_id is required"}, nil
	}

	// 查询合约配置
	contract, err := l.svcCtx.Repo.GetContractConfig(l.ctx, req.ContractConfigID)
	if err != nil {
		l.Logger.Errorf("failed to get contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: "internal error"}, nil
	}
	if contract == nil {
		return &types.CommonResponse{Code: 404, Message: "contract config not found"}, nil
	}

	// 校验租户归属
	if contract.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "contract not owned by current tenant"}, nil
	}

	// 检查是否已开启订阅（重复检测）
	if contract.EnableSubscribe {
		return &types.CommonResponse{Code: 409, Message: "该合约已存在订阅，请勿重复创建"}, nil
	}

	// 更新 DB 字段：开启订阅
	contract.EnableSubscribe = true
	if err := l.svcCtx.Repo.UpdateContractConfig(l.ctx, contract); err != nil {
		l.Logger.Errorf("failed to update contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: "internal error"}, nil
	}

	// 启动链上事件监听（通过清除 SubscribeFlag 让调度器在下一个周期自动拉起）
	flagKey := sdk.SubscribeKeyByID(contract.ChainConfigID, contract.ID)
	sdk.SubscribeFlag.Delete(flagKey)

	// 获取链配置信息用于返回
	chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, contract.ChainConfigID)
	chainName := ""
	if chainConfig != nil {
		chainName = chainConfig.ChainName
	}

	l.Logger.Infof("subscription enabled for contract %d (chain_config_id=%d, tenant=%d)",
		contract.ID, contract.ChainConfigID, tenantID)

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"contract_config_id": contract.ID,
			"chain_config_id":    contract.ChainConfigID,
			"chain_name":         chainName,
			"contract_name":      contract.ContractName,
			"contract_addr":      contract.ContractAddr,
			"enable_subscribe":   true,
		},
	}, nil
}

// ListAvailableContracts 列出当前租户下未开启订阅的合约（供前端选择）
func (l *SubscribeByContractLogic) ListAvailableContracts() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	contracts, err := l.svcCtx.Repo.ListDisabledSubscribeContracts(l.ctx, tenantID)
	if err != nil {
		l.Logger.Errorf("failed to list disabled subscribe contracts: %v", err)
		return &types.CommonResponse{Code: 500, Message: "internal error"}, nil
	}

	var items []map[string]interface{}
	for _, contract := range contracts {
		items = append(items, map[string]interface{}{
			"contract_config_id": contract.ID,
			"chain_config_id":    contract.ChainConfigID,
			"chain_name":         contract.ChainConfig.ChainName,
			"contract_name":      contract.ContractName,
			"contract_addr":      contract.ContractAddr,
		})
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"items": items,
			"total": len(items),
		},
	}, nil
}
