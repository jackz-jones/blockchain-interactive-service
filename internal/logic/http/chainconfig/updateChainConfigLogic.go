package chainconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"
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
	if before == nil {
		return &types.CommonResponse{Code: 404, Message: "chain config not found"}, nil
	}

	// 保存变更前数据用于审计日志（包含链类型专属字段）
	beforeData := map[string]interface{}{
		"chain_name": before.ChainName,
		"chain_type": before.ChainType,
		"enable":     before.Enable,
	}
	// 根据链类型记录专属字段的变更前值
	switch strings.ToLower(before.ChainType) {
	case "chainmaker":
		beforeData["chain_id"] = before.ChainId
		beforeData["auth_type"] = before.AuthType
		beforeData["org_id"] = before.OrgId
		beforeData["hash_type"] = before.HashType
		beforeData["sign_key"] = before.SignKey
		beforeData["sign_cert"] = before.SignCert
		beforeData["user_tls_key"] = before.UserTlsKey
		beforeData["user_tls_cert"] = before.UserTlsCert
		beforeData["user_enc_key"] = before.UserEncKey
		beforeData["user_enc_cert"] = before.UserEncCert
		beforeData["proxy_url"] = before.ProxyUrl
	case "ethereum":
		beforeData["eth_chain_id"] = before.EthChainId
		beforeData["http_url"] = before.HttpUrl
		beforeData["websocket_url"] = before.WebsocketUrl
		beforeData["private_key"] = before.PrivateKey
	case "solana":
		beforeData["sol_rpc_url"] = before.SolRpcUrl
		beforeData["sol_private_key"] = before.SolPrivateKey
		beforeData["commitment_level"] = before.CommitmentLevel
		beforeData["skip_preflight"] = before.SkipPreflight
		beforeData["max_retries"] = before.MaxRetries
	}

	// 在已有记录上修改字段，避免 created_at 被零值覆盖
	if req.ChainName != "" {
		before.ChainName = req.ChainName
	}
	if req.ChainType != "" {
		before.ChainType = strings.ToLower(req.ChainType)
	}
	before.Enable = req.Enable

	// 根据链类型更新对应字段（非空才更新，防止脱敏回显的空值覆盖原有敏感字段）
	// 同时检测脱敏格式（包含 ****），脱敏格式的值也不应覆盖原值
	switch strings.ToLower(before.ChainType) {
	case "chainmaker":
		if req.ChainId != "" {
			before.ChainId = req.ChainId
		}
		if req.AuthType != "" {
			before.AuthType = req.AuthType
		}
		if req.OrgId != "" {
			before.OrgId = req.OrgId
		}
		if req.HashType != "" {
			before.HashType = req.HashType
		}
		if req.SignKey != "" && !isMaskedValue(req.SignKey) {
			before.SignKey = req.SignKey
		}
		if req.SignCert != "" {
			before.SignCert = req.SignCert
		}
		if req.UserTlsKey != "" && !isMaskedValue(req.UserTlsKey) {
			before.UserTlsKey = req.UserTlsKey
		}
		if req.UserTlsCert != "" {
			before.UserTlsCert = req.UserTlsCert
		}
		if req.UserEncKey != "" && !isMaskedValue(req.UserEncKey) {
			before.UserEncKey = req.UserEncKey
		}
		if req.UserEncCert != "" {
			before.UserEncCert = req.UserEncCert
		}
		if req.ProxyUrl != "" {
			before.ProxyUrl = req.ProxyUrl
		}
	case "ethereum":
		if req.EthChainId != 0 {
			before.EthChainId = req.EthChainId
		}
		if req.HttpUrl != "" {
			before.HttpUrl = req.HttpUrl
		}
		if req.WebsocketUrl != "" {
			before.WebsocketUrl = req.WebsocketUrl
		}
		if req.PrivateKey != "" && !isMaskedValue(req.PrivateKey) {
			before.PrivateKey = req.PrivateKey
		}
	case "solana":
		if req.SolRpcUrl != "" {
			before.SolRpcUrl = req.SolRpcUrl
		}
		if req.SolPrivateKey != "" && !isMaskedValue(req.SolPrivateKey) {
			before.SolPrivateKey = req.SolPrivateKey
		}
		if req.CommitmentLevel != "" {
			before.CommitmentLevel = req.CommitmentLevel
		}
		before.SkipPreflight = req.SkipPreflight
		if req.MaxRetries != 0 {
			before.MaxRetries = req.MaxRetries
		}
	}

	// 构建变更后数据，与变更前对比
	afterData := map[string]interface{}{
		"chain_name": before.ChainName,
		"chain_type": before.ChainType,
		"enable":     before.Enable,
	}
	// 根据链类型记录专属字段的变更后值
	switch strings.ToLower(before.ChainType) {
	case "chainmaker":
		afterData["chain_id"] = before.ChainId
		afterData["auth_type"] = before.AuthType
		afterData["org_id"] = before.OrgId
		afterData["hash_type"] = before.HashType
		afterData["sign_key"] = before.SignKey
		afterData["sign_cert"] = before.SignCert
		afterData["user_tls_key"] = before.UserTlsKey
		afterData["user_tls_cert"] = before.UserTlsCert
		afterData["user_enc_key"] = before.UserEncKey
		afterData["user_enc_cert"] = before.UserEncCert
		afterData["proxy_url"] = before.ProxyUrl
	case "ethereum":
		afterData["eth_chain_id"] = before.EthChainId
		afterData["http_url"] = before.HttpUrl
		afterData["websocket_url"] = before.WebsocketUrl
		afterData["private_key"] = before.PrivateKey
	case "solana":
		afterData["sol_rpc_url"] = before.SolRpcUrl
		afterData["sol_private_key"] = before.SolPrivateKey
		afterData["commitment_level"] = before.CommitmentLevel
		afterData["skip_preflight"] = before.SkipPreflight
		afterData["max_retries"] = before.MaxRetries
	}

	// 逐字段比较变更前后数据是否一致
	if util.CompareBeforeAfter(beforeData, afterData) && req.Nodes == nil {
		// 没有实际变更，跳过数据库更新和审计日志记录
		return &types.CommonResponse{Code: 0, Message: "no changes detected", Data: before}, nil
	}

	if err := l.svcCtx.Repo.UpdateChainConfig(l.ctx, before); err != nil {
		return &types.CommonResponse{Code: 500, Message: "update chain config: " + err.Error()}, nil
	}

	// 主动创建包含变更前后对比的审计日志
	auditDetailJSON := util.BuildAuditDetailJSON(beforeData, afterData)

	userID := middleware.GetUserIDFromContext(l.ctx)
	auditLog := &store.AuditLog{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     "update",
		Resource:   "chain_config",
		ResourceID: fmt.Sprintf("%d", req.Id),
		Detail:     auditDetailJSON,
	}

	if err := l.svcCtx.Repo.CreateAuditLog(context.Background(), auditLog); err != nil {
		logx.Errorf("[Audit] failed to record chain config update audit log: %v", err)
	}

	if req.Nodes != nil {
		nodes := buildChainNodesModel(before.ID, req.Nodes)
		if err := l.svcCtx.Repo.ReplaceChainNodes(l.ctx, before.ID, nodes); err != nil {
			return &types.CommonResponse{Code: 500, Message: "replace chain nodes: " + err.Error()}, nil
		}
	}

	l.svcCtx.TenantSDKManager.InvalidateTenantCacheByID(before.ID)

	return &types.CommonResponse{Code: 0, Message: "success", Data: before}, nil
}

// isMaskedValue 检测字符串是否为脱敏格式（包含 ****）
func isMaskedValue(s string) bool {
	return strings.Contains(s, "****")
}
