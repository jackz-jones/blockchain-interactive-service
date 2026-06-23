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

	// 保存变更前数据用于审计日志（abi_json 过长，仅记录是否变更）
	beforeData := map[string]interface{}{
		"contract_name": existing.ContractName,
		"contract_addr": existing.ContractAddr,
	}

	existing.ContractName = req.ContractName
	existing.ContractAddr = req.ContractAddr
	existing.AbiJSON = req.AbiJson
	existing.ExtraConf = req.ExtraConf

	// 构建变更后数据，与变更前对比
	afterData := map[string]interface{}{
		"contract_name": existing.ContractName,
		"contract_addr": existing.ContractAddr,
	}

	// 对大字段仅标记是否变更
	if beforeData["contract_name"] != afterData["contract_name"] ||
		beforeData["contract_addr"] != afterData["contract_addr"] {
		// 已在 before/after 中体现
	}
	// abi_json 和 extra_conf 仅标记变更状态
	if req.AbiJson != "" {
		beforeData["abi_json"] = "(已变更)"
		afterData["abi_json"] = "(已变更)"
	}
	if req.ExtraConf != "" {
		beforeData["extra_conf"] = "(已变更)"
		afterData["extra_conf"] = "(已变更)"
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

	return &types.CommonResponse{Code: 0, Message: "success", Data: existing}, nil
}
