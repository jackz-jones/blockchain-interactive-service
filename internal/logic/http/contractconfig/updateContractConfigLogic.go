package contractconfig

import (
	"context"
	"fmt"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const (
	contractConfigNotFound = "contract config not found"
	contractNameIsRequired = "contract_name is required"
	contractAddr           = "contract_addr"
	contractName           = "contract_name"
	unauthorizedStr        = "unauthorized"
	successStr             = "success"
	changedLabel           = "(已变更)"
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
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: msgAdminRoleRequired}, nil
	}

	existing, err := l.svcCtx.Repo.GetContractConfig(l.ctx, req.Id)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get contract config: " + err.Error()}, nil
	}
	if existing == nil {
		return &types.CommonResponse{Code: 404, Message: contractConfigNotFound}, nil
	}
	if existing.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: msgContractNotOwned}, nil
	}

	if req.ContractName == "" {
		return &types.CommonResponse{Code: 400, Message: contractNameIsRequired}, nil
	}

	// 保存变更前数据用于审计日志（abi_json 过长，仅记录是否变更）
	beforeData := map[string]interface{}{
		contractName: existing.ContractName,
		contractAddr: existing.ContractAddr,
	}

	existing.ContractName = req.ContractName
	existing.ContractAddr = req.ContractAddr
	existing.AbiJSON = req.AbiJson
	// 注意：EnableSubscribe 和 ExtraConf 中的订阅参数不在合约管理页面修改
	// 订阅相关参数的修改统一在订阅管理页面进行
	// existing.EnableSubscribe 和 existing.ExtraConf 保持原值不变

	// 构建变更后数据，与变更前对比
	afterData := map[string]interface{}{
		contractName: existing.ContractName,
		contractAddr: existing.ContractAddr,
	}

	// 对大字段（contract_name / contract_addr / abi_json）仅标记变更状态，不再展开内容
	// abi_json 仅标记变更状态
	if req.AbiJson != "" {
		beforeData["abi_json"] = changedLabel
		afterData["abi_json"] = changedLabel
	}

	// 逐字段比较变更前后数据是否一致
	if util.CompareBeforeAfter(beforeData, afterData) {
		// 没有实际变更，跳过数据库更新和审计日志记录
		return &types.CommonResponse{Code: 0, Message: "no changes detected", Data: existing}, nil
	}

	if err := l.svcCtx.Repo.UpdateContractConfig(l.ctx, existing); err != nil {
		return &types.CommonResponse{Code: 500, Message: "update contract config: " + err.Error()}, nil
	}

	// 主动创建包含变更前后对比的审计日志
	auditDetailJSON := util.BuildAuditDetailJSON(beforeData, afterData)

	userID := middleware.GetUserIDFromContext(l.ctx)
	auditLog := &store.AuditLog{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     "update",
		Resource:   "contract_config",
		ResourceID: fmt.Sprintf("%d", req.Id),
		Detail:     auditDetailJSON,
	}

	if err := l.svcCtx.Repo.CreateAuditLog(context.Background(), auditLog); err != nil {
		logx.Errorf("[Audit] failed to record contract config update audit log: %v", err)
	}

	// 若该合约已开启订阅，则中断对应的订阅协程，调度器将在下一轮询周期基于新配置自动重启
	// （修改 ABI 后需要重启订阅以应用新的合约接口定义，但订阅参数不变）
	if existing.EnableSubscribe {
		chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.ChainConfigId)
		if chainConfig != nil {
			l.svcCtx.TenantSDKManager.StopContractSubscription(
				req.ChainConfigId, req.Id, tenantID, chainConfig.ChainName,
			)
			logx.Infof("[ContractConfig] interrupted subscription for contract update: chainConfigID=%d, contractConfigID=%d",
				req.ChainConfigId, req.Id)
		}
	} else {
		// 合约未开启订阅，仅使 SDK 缓存失效以刷新合约配置
		chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.ChainConfigId)
		if chainConfig != nil {
			l.svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, chainConfig.ChainName)
		}
	}

	// 使 ConfigResolver 缓存失效，避免 ResolveByName/ResolveByAddr 返回过时的合约地址/ABI
	if l.svcCtx.ConfigResolver != nil {
		l.svcCtx.ConfigResolver.InvalidateByChainConfigID(req.ChainConfigId)
	}

	return &types.CommonResponse{Code: 0, Message: successStr, Data: existing}, nil
}
