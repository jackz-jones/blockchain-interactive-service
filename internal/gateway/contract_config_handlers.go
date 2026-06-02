package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

// ========== 合约配置管理相关 Handler ==========

// ContractConfigRequest 合约配置请求体
type ContractConfigRequest struct {
	ContractName    string `json:"contract_name"`    // 合约名称（必填）
	ContractAddr    string `json:"contract_addr"`    // 合约地址
	AbiJSON         string `json:"abi_json"`         // ABI JSON
	EnableSubscribe bool   `json:"enable_subscribe"` // 是否开启事件订阅
	ExtraConf       string `json:"extra_conf"`       // 额外配置 JSON（包含 deployBlockHeight 等）
}

// ContractConfigResponse 合约配置响应体
type ContractConfigResponse struct {
	ID              uint   `json:"id"`
	TenantID        uint   `json:"tenant_id"`
	ChainConfigID   uint   `json:"chain_config_id"`
	ContractName    string `json:"contract_name"`
	ContractAddr    string `json:"contract_addr"`
	AbiJSON         string `json:"abi_json"`
	EnableSubscribe bool   `json:"enable_subscribe"`
	ExtraConf       string `json:"extra_conf"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// toContractConfigResponse 将数据库模型转换为响应体
func toContractConfigResponse(config *store.TenantContractConfig) *ContractConfigResponse {
	return &ContractConfigResponse{
		ID:              config.ID,
		TenantID:        config.TenantID,
		ChainConfigID:   config.ChainConfigID,
		ContractName:    config.ContractName,
		ContractAddr:    config.ContractAddr,
		AbiJSON:         config.AbiJSON,
		EnableSubscribe: config.EnableSubscribe,
		ExtraConf:       config.ExtraConf,
		CreatedAt:       config.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       config.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// getChainConfigIDFromPath 从路径参数中获取 chainConfigId 并校验归属权限
func getChainConfigIDFromPath(
	r *http.Request, svcCtx *svc.ServiceContext, tenantID uint,
) (uint, *store.TenantChainConfig, error) {
	vars := pathvar.Vars(r)
	idStr := vars["chainConfigId"]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, nil, err
	}

	// 查询链配置并校验归属
	chainConfigs, err := svcCtx.Repo.ListChainConfigsByTenant(r.Context(), tenantID)
	if err != nil {
		return 0, nil, err
	}

	for _, cc := range chainConfigs {
		if cc.ID == uint(id) {
			return uint(id), cc, nil
		}
	}

	return 0, nil, nil // 未找到或不属于当前租户
}

// CreateContractConfigHandler 创建合约配置 Handler
func CreateContractConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 权限校验：仅 admin 可写
		role := middleware.GetUserRoleFromHTTP(r)
		if role != store.UserRoleAdmin {
			errorResponse(w, http.StatusForbidden, "admin role required for write operations")
			return
		}

		// 获取并校验 chainConfigId 归属
		chainConfigID, chainConfig, err := getChainConfigIDFromPath(r, svcCtx, tenantID)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid chain_config_id")
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusForbidden, "chain config not found or not owned by current tenant")
			return
		}

		// 前置依赖校验：链配置必须启用
		if !chainConfig.Enable {
			errorResponse(w, http.StatusBadRequest, "关联的链配置未启用，请先启用链配置")
			return
		}

		// 解析请求体
		var req ContractConfigRequest
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		// 必填字段校验
		if req.ContractName == "" {
			errorResponse(w, http.StatusBadRequest, "contract_name is required")
			return
		}

		// 字段校验（根据链类型）
		if err := ValidateContractConfig(chainConfig.ChainType, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// 检查合约名称唯一性
		existingConfigs, err := svcCtx.Repo.ListContractConfigsByChain(r.Context(), chainConfigID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "check contract name: "+err.Error())
			return
		}
		for _, ec := range existingConfigs {
			if ec.ContractName == req.ContractName {
				errorResponse(w, http.StatusConflict, "contract name already exists under this chain config")
				return
			}
		}

		// 创建合约配置
		config := &store.TenantContractConfig{
			TenantID:        tenantID,
			ChainConfigID:   chainConfigID,
			ContractName:    req.ContractName,
			ContractAddr:    req.ContractAddr,
			AbiJSON:         req.AbiJSON,
			EnableSubscribe: req.EnableSubscribe,
			ExtraConf:       req.ExtraConf,
		}

		if err := svcCtx.Repo.CreateContractConfig(r.Context(), config); err != nil {
			errorResponse(w, http.StatusInternalServerError, "create contract config: "+err.Error())
			return
		}

		// 使 SDK 缓存失效
		svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, chainConfig.ChainName)

		// 记录审计日志
		recordConfigAuditLog(svcCtx, r, tenantID, "create", "contract_config", config.ID, nil, config)

		successResponse(w, toContractConfigResponse(config))
	}
}

// ListContractConfigsHandler 列出合约配置 Handler
func ListContractConfigsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 获取并校验 chainConfigId 归属
		chainConfigID, chainConfig, err := getChainConfigIDFromPath(r, svcCtx, tenantID)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid chain_config_id")
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusForbidden, "chain config not found or not owned by current tenant")
			return
		}

		configs, err := svcCtx.Repo.ListContractConfigsByChain(r.Context(), chainConfigID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list contract configs: "+err.Error())
			return
		}

		// 转换为响应格式
		var items []*ContractConfigResponse
		for _, c := range configs {
			items = append(items, toContractConfigResponse(c))
		}

		successResponse(w, map[string]interface{}{
			"total": len(items),
			"items": items,
		})
	}
}

// GetContractConfigHandler 获取单个合约配置 Handler
func GetContractConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 获取并校验 chainConfigId 归属
		_, chainConfig, err := getChainConfigIDFromPath(r, svcCtx, tenantID)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid chain_config_id")
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusForbidden, "chain config not found or not owned by current tenant")
			return
		}

		// 获取合约配置 ID
		vars := pathvar.Vars(r)
		contractIDStr := vars["id"]
		contractID, err := strconv.ParseUint(contractIDStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid contract config id")
			return
		}

		config, err := svcCtx.Repo.GetContractConfig(r.Context(), uint(contractID))
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get contract config: "+err.Error())
			return
		}
		if config == nil {
			errorResponse(w, http.StatusNotFound, "contract config not found")
			return
		}

		// 校验合约配置归属
		if config.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "contract config not owned by current tenant")
			return
		}

		successResponse(w, toContractConfigResponse(config))
	}
}

// UpdateContractConfigHandler 更新合约配置 Handler
func UpdateContractConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 权限校验：仅 admin 可写
		role := middleware.GetUserRoleFromHTTP(r)
		if role != store.UserRoleAdmin {
			errorResponse(w, http.StatusForbidden, "admin role required for write operations")
			return
		}

		// 获取并校验 chainConfigId 归属
		_, chainConfig, err := getChainConfigIDFromPath(r, svcCtx, tenantID)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid chain_config_id")
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusForbidden, "chain config not found or not owned by current tenant")
			return
		}

		// 获取合约配置 ID
		vars := pathvar.Vars(r)
		contractIDStr := vars["id"]
		contractID, err := strconv.ParseUint(contractIDStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid contract config id")
			return
		}

		// 查询现有配置
		existing, err := svcCtx.Repo.GetContractConfig(r.Context(), uint(contractID))
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get contract config: "+err.Error())
			return
		}
		if existing == nil {
			errorResponse(w, http.StatusNotFound, "contract config not found")
			return
		}
		if existing.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "contract config not owned by current tenant")
			return
		}

		// 解析请求体
		var req ContractConfigRequest
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if req.ContractName == "" {
			errorResponse(w, http.StatusBadRequest, "contract_name is required")
			return
		}

		// 字段校验（根据链类型）
		if err := ValidateContractConfig(chainConfig.ChainType, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// 保存变更前快照
		beforeSnapshot := *existing

		// 更新字段
		existing.ContractName = req.ContractName
		existing.ContractAddr = req.ContractAddr
		existing.AbiJSON = req.AbiJSON
		existing.ExtraConf = req.ExtraConf

		if err := svcCtx.Repo.UpdateContractConfig(r.Context(), existing); err != nil {
			errorResponse(w, http.StatusInternalServerError, "update contract config: "+err.Error())
			return
		}

		// 使 SDK 缓存失效
		svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, chainConfig.ChainName)

		// 检查订阅状态变更
		handleSubscriptionChange(svcCtx, tenantID, chainConfig, &beforeSnapshot, existing)

		// 记录审计日志
		recordConfigAuditLog(svcCtx, r, tenantID, "update", "contract_config", existing.ID, &beforeSnapshot, existing)

		successResponse(w, toContractConfigResponse(existing))
	}
}

// DeleteContractConfigHandler 删除合约配置 Handler
func DeleteContractConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 权限校验：仅 admin 可写
		role := middleware.GetUserRoleFromHTTP(r)
		if role != store.UserRoleAdmin {
			errorResponse(w, http.StatusForbidden, "admin role required for write operations")
			return
		}

		// 获取并校验 chainConfigId 归属
		_, chainConfig, err := getChainConfigIDFromPath(r, svcCtx, tenantID)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid chain_config_id")
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusForbidden, "chain config not found or not owned by current tenant")
			return
		}

		// 获取合约配置 ID
		vars := pathvar.Vars(r)
		contractIDStr := vars["id"]
		contractID, err := strconv.ParseUint(contractIDStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid contract config id")
			return
		}

		// 查询现有配置（用于审计日志）
		existing, err := svcCtx.Repo.GetContractConfig(r.Context(), uint(contractID))
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get contract config: "+err.Error())
			return
		}
		if existing == nil {
			errorResponse(w, http.StatusNotFound, "contract config not found")
			return
		}
		if existing.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "contract config not owned by current tenant")
			return
		}

		// 软删除
		if err := svcCtx.Repo.DeleteContractConfig(r.Context(), uint(contractID)); err != nil {
			errorResponse(w, http.StatusInternalServerError, "delete contract config: "+err.Error())
			return
		}

		// 使 SDK 缓存失效
		svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, chainConfig.ChainName)

		// 记录审计日志
		recordConfigAuditLog(svcCtx, r, tenantID, "delete", "contract_config", existing.ID, existing, nil)

		successResponse(w, nil)
	}
}

// handleSubscriptionChange 处理订阅状态变更
func handleSubscriptionChange(svcCtx *svc.ServiceContext, tenantID uint,
	chainConfig *store.TenantChainConfig, before, after *store.TenantContractConfig) {

	// 解析 ExtraConf 中的 enableSubscribe 字段
	type extraConfFields struct {
		EnableSubscribe bool `json:"enable_subscribe"`
	}

	var beforeExtra, afterExtra extraConfFields
	if before.ExtraConf != "" {
		_ = json.Unmarshal([]byte(before.ExtraConf), &beforeExtra)
	}
	if after.ExtraConf != "" {
		_ = json.Unmarshal([]byte(after.ExtraConf), &afterExtra)
	}

	// 订阅状态发生变更时，缓存失效已经处理了，下次重建客户端时会按新配置启动/停止订阅
	// 这里只需要确保缓存已失效（上层已调用 InvalidateTenantCache）
	_ = beforeExtra
	_ = afterExtra
}

// recordConfigAuditLog 记录配置操作审计日志
func recordConfigAuditLog(svcCtx *svc.ServiceContext, r *http.Request,
	tenantID uint, action, resourceType string, resourceID uint, before, after interface{}) {

	userID := middleware.GetUserIDFromHTTP(r)

	detail := map[string]interface{}{
		"resource_type": resourceType,
		"resource_id":   resourceID,
	}
	if before != nil {
		detail["before"] = before
	}
	if after != nil {
		detail["after"] = after
	}

	detailJSON, _ := json.Marshal(detail)

	auditLog := &store.AuditLog{
		TenantID:  tenantID,
		UserID:    userID,
		Action:    action + "_" + resourceType,
		Resource:  resourceType,
		Detail:    string(detailJSON),
		IP:        getClientIPFromHTTP(r),
		UserAgent: r.UserAgent(),
	}

	// 异步写入审计日志
	go func() {
		_ = svcCtx.Repo.CreateAuditLog(r.Context(), auditLog)
	}()
}
