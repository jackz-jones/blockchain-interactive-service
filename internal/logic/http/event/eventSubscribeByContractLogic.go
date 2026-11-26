package event

import (
	"context"
	"encoding/json"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const (
	chainConfigId = "chain_config_id"
	contractAddr  = "contract_addr"
	contractName  = "contract_name"
	chainNameStr  = "chain_name"
	chainType     = "chain_type"
	totalStr      = "total"
	itemsStr      = "items"
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

func (l *SubscribeByContractLogic) SubscribeByContract(
	req *types.SubscribeByContractRequest,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	if req.ContractConfigID == 0 {
		return &types.CommonResponse{Code: 400, Message: "contract_config_id is required"}, nil
	}

	// 查询合约配置
	contract, err := l.svcCtx.Repo.GetContractConfig(l.ctx, req.ContractConfigID)
	if err != nil {
		l.Errorf("failed to get contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: internalError}, nil
	}
	if contract == nil {
		return &types.CommonResponse{Code: 404, Message: contractConfigNotFound}, nil
	}

	// 校验租户归属
	if contract.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: msgContractNotOwnedEvent}, nil
	}

	// 检查是否已开启订阅（重复检测）
	if contract.EnableSubscribe {
		return &types.CommonResponse{Code: 409, Message: "该合约已存在订阅，请勿重复创建"}, nil
	}

	// 更新 DB 字段：开启订阅
	contract.EnableSubscribe = true

	// 将订阅参数写入 extra_conf
	extraConf := map[string]interface{}{}
	if contract.ExtraConf != "" {
		_ = json.Unmarshal([]byte(contract.ExtraConf), &extraConf)
	}
	if req.DeployBlockHeight != nil {
		extraConf["DeployBlockHeight"] = *req.DeployBlockHeight
	}
	if req.GetHistoryEventInterval != nil {
		extraConf["GetHistoryEventInterval"] = *req.GetHistoryEventInterval
	}
	if req.GetHistoryEventHeightWindow != nil {
		extraConf["GetHistoryEventHeightWindow"] = *req.GetHistoryEventHeightWindow
	}
	if extraConfBytes, err := json.Marshal(extraConf); err == nil {
		contract.ExtraConf = string(extraConfBytes)
	}

	if err := l.svcCtx.Repo.UpdateContractConfig(l.ctx, contract); err != nil {
		l.Errorf("failed to update contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: internalError}, nil
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

	l.Infof("subscription enabled for contract %d (chain_config_id=%d, tenant=%d)",
		contract.ID, contract.ChainConfigID, tenantID)

	return &types.CommonResponse{
		Code:    0,
		Message: successStr,
		Data: map[string]interface{}{
			contractConfigId: contract.ID,
			chainConfigId:    contract.ChainConfigID,
			chainNameStr:     chainName,
			contractName:     contract.ContractName,
			contractAddr:     contract.ContractAddr,
			enableSubscribe:  true,
		},
	}, nil
}

// ListAvailableContracts 列出当前租户下未开启订阅的合约（供前端选择）
func (l *SubscribeByContractLogic) ListAvailableContracts() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	contracts, err := l.svcCtx.Repo.ListDisabledSubscribeContracts(l.ctx, tenantID)
	if err != nil {
		l.Errorf("failed to list disabled subscribe contracts: %v", err)
		return &types.CommonResponse{Code: 500, Message: internalError}, nil
	}

	var items []map[string]interface{}
	for _, contract := range contracts {
		items = append(items, map[string]interface{}{
			contractConfigId: contract.ID,
			chainConfigId:    contract.ChainConfigID,
			chainNameStr:     contract.ChainConfig.ChainName,
			chainType:        contract.ChainConfig.ChainType,
			contractName:     contract.ContractName,
			contractAddr:     contract.ContractAddr,
		})
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	return &types.CommonResponse{
		Code:    0,
		Message: successStr,
		Data: map[string]interface{}{
			itemsStr: items,
			totalStr: len(items),
		},
	}, nil
}
