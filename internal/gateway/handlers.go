package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

// 常量定义
const (
	// sourceDatabase 数据来源标识：数据库
	sourceDatabase = "database"
)

// JSON 响应辅助函数
func jsonResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, code int, msg string) {
	jsonResponse(w, code, map[string]interface{}{
		"code":    code,
		"message": msg,
	})
}

func successResponse(w http.ResponseWriter, data interface{}) {
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// ========== 合约调用相关 Handler ==========

// CallContractRequest HTTP 合约调用请求体
type CallContractRequest struct {
	ChainName    string            `json:"chain_name"`
	ContractName string            `json:"contract_name"`
	Method       string            `json:"method"`
	Params       map[string]string `json:"params"`
}

// CallContractHandler 合约调用 Handler
func CallContractHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CallContractRequest
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if req.ChainName == "" || req.ContractName == "" || req.Method == "" {
			errorResponse(w, http.StatusBadRequest, "chain_name, contract_name and method are required")
			return
		}

		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 获取租户级 SDK 客户端
		client, err := svcCtx.TenantSDKManager.GetTenantSDKClient(r.Context(), tenantID, req.ChainName)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "get sdk client: "+err.Error())
			return
		}

		// 构建参数
		var kvPairs []*pb.KeyValuePair
		for k, v := range req.Params {
			kvPairs = append(kvPairs, &pb.KeyValuePair{Key: k, Value: []byte(v)})
		}

		// 调用合约（默认 Invoke 类型，超时 10s，同步等待结果）
		start := time.Now()
		txId, result, err := client.CallContract(pb.MethodType_Invoke, req.ContractName, req.Method, kvPairs, 10, true)
		duration := time.Since(start)

		if err != nil {
			// 记录调用日志
			go recordCallLog(svcCtx, tenantID, r, req, "failed", err.Error(), 0, duration)
			errorResponse(w, http.StatusInternalServerError, "invoke contract: "+err.Error())
			return
		}

		// 记录调用日志
		go recordCallLog(svcCtx, tenantID, r, req, "success", "", 0, duration)

		successResponse(w, map[string]interface{}{
			"tx_id":    txId,
			"result":   result,
			"duration": duration.Milliseconds(),
		})
	}
}

// GetTxByTxIdHandler 查询交易 Handler
func GetTxByTxIdHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := pathvar.Vars(r)
		txId := vars["txId"]
		chainName := r.URL.Query().Get("chain_name")

		if txId == "" || chainName == "" {
			errorResponse(w, http.StatusBadRequest, "txId path param and chain_name query param are required")
			return
		}

		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		client, err := svcCtx.TenantSDKManager.GetTenantSDKClient(r.Context(), tenantID, chainName)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "get sdk client: "+err.Error())
			return
		}

		result, confirmed, err := client.GetTxByTxId(txId)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get tx: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"result":    result,
			"confirmed": confirmed,
		})
	}
}

// GetAvailableChainsHandler 获取可用链列表 Handler（从数据库加载租户链配置）
func GetAvailableChainsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 从数据库加载租户链配置
		dbConfigs, err := svcCtx.Repo.ListChainConfigsByTenant(r.Context(), tenantID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list chain configs: "+err.Error())
			return
		}

		// 收集结果
		chains := make([]map[string]interface{}, 0)
		for _, dbConf := range dbConfigs {
			if !dbConf.Enable {
				continue
			}

			// 获取合约配置
			contractConfigs, _ := svcCtx.Repo.ListContractConfigsByChain(r.Context(), dbConf.ID)
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
			status := svcCtx.TenantSDKManager.GetClientStatus(tenantID, dbConf.ChainName)
			if active, ok := status["client_active"].(bool); ok {
				chainInfo["client_active"] = active
			}

			chains = append(chains, chainInfo)
		}

		successResponse(w, map[string]interface{}{
			"chains": chains,
			"total":  len(chains),
		})
	}
}

// GetChainStatusHandler 获取链运行状态 Handler
func GetChainStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		vars := pathvar.Vars(r)
		chainName := vars["chainName"]
		if chainName == "" {
			errorResponse(w, http.StatusBadRequest, "chainName is required")
			return
		}

		// 获取客户端状态
		status := svcCtx.TenantSDKManager.GetClientStatus(tenantID, chainName)

		// 获取链配置信息
		chainConfig, err := svcCtx.Repo.GetChainConfig(r.Context(), tenantID, chainName)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get chain config: "+err.Error())
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusNotFound, "chain not found for this tenant")
			return
		}

		// 获取合约配置列表及订阅状态
		contractConfigs, _ := svcCtx.Repo.ListContractConfigsByChain(r.Context(), chainConfig.ID)
		var contractStatuses []map[string]interface{}
		for _, cc := range contractConfigs {
			cs := map[string]interface{}{
				"contract_name": cc.ContractName,
				"contract_addr": cc.ContractAddr,
			}
			contractStatuses = append(contractStatuses, cs)
		}

		status["chain_type"] = chainConfig.ChainType
		status["enable"] = chainConfig.Enable
		status["source"] = sourceDatabase
		status["contracts"] = contractStatuses

		successResponse(w, status)
	}
}

// ========== 租户管理相关 Handler ==========

// CreateTenantRequestBody 创建租户请求体
type CreateTenantRequestBody struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Plan     string `json:"plan"`
	Password string `json:"password"`
}

// CreateTenantHandler 创建租户 Handler
func CreateTenantHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateTenantRequestBody
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if req.Name == "" || req.Email == "" || req.Password == "" {
			errorResponse(w, http.StatusBadRequest, "name, email and password are required")
			return
		}

		resp, err := svcCtx.TenantService.CreateTenant(r.Context(), &tenant.CreateTenantRequest{
			Name:     req.Name,
			Email:    req.Email,
			Phone:    req.Phone,
			Plan:     req.Plan,
			Password: req.Password,
		})
		if err != nil {
			if err == tenant.ErrTenantExists {
				errorResponse(w, http.StatusConflict, "tenant already exists")
				return
			}
			errorResponse(w, http.StatusInternalServerError, "create tenant: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"tenant_id": resp.Tenant.ID,
			"api_key":   resp.APIKey.Key,
			"username":  resp.Admin.Username,
		})
	}
}

// GetTenantHandler 获取租户信息 Handler
func GetTenantHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := pathvar.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid tenant id")
			return
		}

		t, err := svcCtx.TenantService.GetTenant(r.Context(), uint(id))
		if err != nil {
			if err == tenant.ErrTenantNotFound {
				errorResponse(w, http.StatusNotFound, "tenant not found")
				return
			}
			errorResponse(w, http.StatusInternalServerError, "get tenant: "+err.Error())
			return
		}

		successResponse(w, t)
	}
}

// ========== API Key 管理相关 Handler ==========

// CreateAPIKeyRequestBody 创建 API Key 请求体
type CreateAPIKeyRequestBody struct {
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
	IPWhitelist string `json:"ip_whitelist"`
	ExpiresIn   int    `json:"expires_in"` // 过期时间（小时），0 表示永不过期
}

// CreateAPIKeyHandler 创建 API Key Handler
func CreateAPIKeyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateAPIKeyRequestBody
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		tenantID := middleware.GetTenantIDFromHTTP(r)
		userID := middleware.GetUserIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var expiresAt *time.Time
		if req.ExpiresIn > 0 {
			t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
			expiresAt = &t
		}

		apiKey, err := svcCtx.TenantService.CreateAPIKey(r.Context(), &tenant.CreateAPIKeyRequest{
			TenantID:    tenantID,
			UserID:      userID,
			Name:        req.Name,
			Permissions: req.Permissions,
			IPWhitelist: req.IPWhitelist,
			ExpiresAt:   expiresAt,
		})
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "create api key: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"id":  apiKey.ID,
			"key": apiKey.Key,
		})
	}
}

// ListAPIKeysHandler 列出 API Key Handler
func ListAPIKeysHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}

		keys, total, err := svcCtx.Repo.ListAPIKeysByTenant(r.Context(), tenantID, (page-1)*pageSize, pageSize)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list api keys: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"total": total,
			"items": keys,
		})
	}
}

// ========== 链配置管理相关 Handler ==========

// NodeRequest 节点配置请求体
type NodeRequest struct {
	NodeAddr    string `json:"node_addr"`
	ConnCnt     int    `json:"conn_cnt"`
	EnableTls   bool   `json:"enable_tls"`
	TlsHostName string `json:"tls_host_name"`
	CaCert      string `json:"ca_cert"`
}

// CreateChainConfigRequestBody 创建链配置请求体
type CreateChainConfigRequestBody struct {
	ChainName string `json:"chain_name"`
	ChainType string `json:"chain_type"`
	Enable    bool   `json:"enable"`

	// ChainMaker 专属字段
	ChainId     string `json:"chain_id"`
	AuthType    string `json:"auth_type"`
	OrgId       string `json:"org_id"`
	HashType    string `json:"hash_type"`
	SignKey     string `json:"sign_key"`
	SignCert    string `json:"sign_cert"`
	UserTlsKey  string `json:"user_tls_key"`
	UserTlsCert string `json:"user_tls_cert"`
	UserEncKey  string `json:"user_enc_key"`
	UserEncCert string `json:"user_enc_cert"`
	ProxyUrl    string `json:"proxy_url"`

	// Ethereum 专属字段
	EthChainId   int64  `json:"eth_chain_id"`
	HttpUrl      string `json:"http_url"`
	WebsocketUrl string `json:"websocket_url"`
	PrivateKey   string `json:"private_key"`
	GasLimit     int64  `json:"gas_limit"`

	// Solana 专属字段
	SolRpcUrl       string `json:"sol_rpc_url"`
	SolPrivateKey   string `json:"sol_private_key"`
	CommitmentLevel string `json:"commitment_level"`
	SkipPreflight   bool   `json:"skip_preflight"`
	MaxRetries      int    `json:"max_retries"`

	// 节点配置（ChainMaker 专用）
	Nodes []NodeRequest `json:"nodes"`
}

// CreateChainConfigHandler 创建链配置 Handler
func CreateChainConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateChainConfigRequestBody
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

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

		if req.ChainName == "" || req.ChainType == "" {
			errorResponse(w, http.StatusBadRequest, "chain_name and chain_type are required")
			return
		}

		// 校验链类型是否合法
		if err := ValidateChainType(req.ChainType); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// 校验链配置字段
		if err := ValidateChainConfig(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// 校验链名称唯一性
		unique, err := svcCtx.Repo.CheckChainNameUnique(r.Context(), tenantID, req.ChainName, 0)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "check chain name: "+err.Error())
			return
		}
		if !unique {
			errorResponse(w, http.StatusConflict, "chain_name already exists for this tenant")
			return
		}

		// 构建链配置模型
		config := buildChainConfigFromRequest(tenantID, &req)

		if err := svcCtx.Repo.CreateChainConfig(r.Context(), config); err != nil {
			errorResponse(w, http.StatusInternalServerError, "create chain config: "+err.Error())
			return
		}

		// 创建节点配置
		if len(req.Nodes) > 0 {
			nodes := buildChainNodesFromRequest(config.ID, req.Nodes)
			if err := svcCtx.Repo.CreateChainNodes(r.Context(), nodes); err != nil {
				errorResponse(w, http.StatusInternalServerError, "create chain nodes: "+err.Error())
				return
			}
		}

		// 记录审计日志
		recordConfigAuditLog(svcCtx, r, tenantID, "create", "chain_config", config.ID, nil, config)

		successResponse(w, config)
	}
}

// ListChainConfigsHandler 列出链配置 Handler
func ListChainConfigsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		configs, err := svcCtx.Repo.ListChainConfigsByTenant(r.Context(), tenantID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list chain configs: "+err.Error())
			return
		}

		// 对敏感字段进行脱敏
		maskedConfigs := make([]*store.TenantChainConfig, 0, len(configs))
		for _, cfg := range configs {
			maskedConfigs = append(maskedConfigs, MaskChainConfigSensitiveFields(cfg))
		}

		successResponse(w, maskedConfigs)
	}
}

// UpdateChainConfigHandler 更新链配置 Handler
func UpdateChainConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := pathvar.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid id")
			return
		}

		var req CreateChainConfigRequestBody
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

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

		// 校验链类型
		if req.ChainType != "" {
			if err := ValidateChainType(req.ChainType); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		// 校验链配置字段
		if req.ChainType != "" {
			if err := ValidateChainConfig(&req); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		// 获取变更前快照
		before, _ := svcCtx.Repo.GetChainConfigByID(r.Context(), uint(id))
		if before != nil && before.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "chain config not owned by current tenant")
			return
		}

		// 构建链配置模型
		config := buildChainConfigFromRequest(tenantID, &req)
		config.ID = uint(id)

		if err := svcCtx.Repo.UpdateChainConfig(r.Context(), config); err != nil {
			errorResponse(w, http.StatusInternalServerError, "update chain config: "+err.Error())
			return
		}

		// 全量替换节点列表
		if req.Nodes != nil {
			nodes := buildChainNodesFromRequest(config.ID, req.Nodes)
			if err := svcCtx.Repo.ReplaceChainNodes(r.Context(), config.ID, nodes); err != nil {
				errorResponse(w, http.StatusInternalServerError, "replace chain nodes: "+err.Error())
				return
			}
		}

		// 使缓存失效，下次请求时会重新加载
		svcCtx.TenantSDKManager.InvalidateTenantCacheByID(config.ID)

		// 记录审计日志
		recordConfigAuditLog(svcCtx, r, tenantID, "update", "chain_config", config.ID, before, config)

		successResponse(w, config)
	}
}

// DeleteChainConfigHandler 删除链配置 Handler
func DeleteChainConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := pathvar.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid id")
			return
		}

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

		// 获取变更前快照
		before, _ := svcCtx.Repo.GetChainConfigByID(r.Context(), uint(id))
		if before != nil && before.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "chain config not owned by current tenant")
			return
		}

		if err := svcCtx.Repo.DeleteChainConfig(r.Context(), uint(id)); err != nil {
			errorResponse(w, http.StatusInternalServerError, "delete chain config: "+err.Error())
			return
		}

		// 停止该链下所有订阅并使缓存失效
		if before != nil {
			svcCtx.TenantSDKManager.StopAllSubscriptions(tenantID, before.ChainName)
		} else {
			svcCtx.TenantSDKManager.InvalidateAllTenantCache(tenantID)
		}

		// 记录审计日志
		recordConfigAuditLog(svcCtx, r, tenantID, "delete", "chain_config", uint(id), before, nil)

		successResponse(w, nil)
	}
}

// GetChainConfigDetailHandler 获取链配置详情（含关联节点和合约配置列表）Handler
func GetChainConfigDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		vars := pathvar.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid id")
			return
		}

		// 查询链配置
		chainConfig, err := svcCtx.Repo.GetChainConfigByID(r.Context(), uint(id))
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get chain config: "+err.Error())
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusNotFound, "chain config not found")
			return
		}
		if chainConfig.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "chain config not owned by current tenant")
			return
		}

		// 查询关联的节点配置
		nodes, err := svcCtx.Repo.ListChainNodesByConfigID(r.Context(), chainConfig.ID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list chain nodes: "+err.Error())
			return
		}

		// 查询关联的合约配置
		contractConfigs, err := svcCtx.Repo.ListContractConfigsByChain(r.Context(), chainConfig.ID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list contract configs: "+err.Error())
			return
		}

		var contracts []*ContractConfigResponse
		for _, c := range contractConfigs {
			contracts = append(contracts, toContractConfigResponse(c))
		}

		// 获取客户端状态
		clientStatus := svcCtx.TenantSDKManager.GetClientStatus(tenantID, chainConfig.ChainName)

		// 对敏感字段进行脱敏
		maskedConfig := MaskChainConfigSensitiveFields(chainConfig)

		successResponse(w, map[string]interface{}{
			"config":        maskedConfig,
			"nodes":         nodes,
			"contracts":     contracts,
			"client_status": clientStatus,
		})
	}
}

// TestChainConnectionHandler 测试链连接 Handler
func TestChainConnectionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 权限校验：仅 admin 可操作
		role := middleware.GetUserRoleFromHTTP(r)
		if role != store.UserRoleAdmin {
			errorResponse(w, http.StatusForbidden, "admin role required")
			return
		}

		vars := pathvar.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid id")
			return
		}

		// 查询链配置
		chainConfig, err := svcCtx.Repo.GetChainConfigByID(r.Context(), uint(id))
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get chain config: "+err.Error())
			return
		}
		if chainConfig == nil {
			errorResponse(w, http.StatusNotFound, "chain config not found")
			return
		}
		if chainConfig.TenantID != tenantID {
			errorResponse(w, http.StatusForbidden, "chain config not owned by current tenant")
			return
		}

		// 尝试创建 SDK 客户端来测试连接
		_, testErr := svcCtx.TenantSDKManager.GetTenantSDKClient(r.Context(), tenantID, chainConfig.ChainName)

		result := map[string]interface{}{
			"chain_name": chainConfig.ChainName,
			"chain_type": chainConfig.ChainType,
			"success":    testErr == nil,
		}
		if testErr != nil {
			result["error"] = testErr.Error()
		}

		// 记录审计日志
		recordConfigAuditLog(svcCtx, r, tenantID, "test_connection", "chain_config", chainConfig.ID, nil, result)

		successResponse(w, result)
	}
}

// ========== 辅助函数 ==========

// recordCallLog 异步记录调用日志
func recordCallLog(svcCtx *svc.ServiceContext, tenantID uint, r *http.Request,
	req CallContractRequest, status, errMsg string, gasUsed uint64, duration time.Duration) {

	userID := middleware.GetUserIDFromHTTP(r)
	apiKeyID := middleware.GetAPIKeyIDFromHTTP(r)

	log := &store.CallLog{
		TenantID:     tenantID,
		UserID:       userID,
		APIKeyID:     apiKeyID,
		ChainName:    req.ChainName,
		Method:       req.Method,
		ContractName: req.ContractName,
		Status:       status,
		ErrorMsg:     errMsg,
		GasUsed:      gasUsed,
		Duration:     duration.Milliseconds(),
		RequestIP:    getClientIPFromHTTP(r),
	}

	_ = svcCtx.Repo.CreateCallLog(r.Context(), log)
}

// getClientIPFromHTTP 从 HTTP 请求中获取客户端 IP
func getClientIPFromHTTP(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 从 RemoteAddr 获取
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// buildChainConfigFromRequest 从请求体构建链配置模型
func buildChainConfigFromRequest(tenantID uint, req *CreateChainConfigRequestBody) *store.TenantChainConfig {
	config := &store.TenantChainConfig{
		TenantID:  tenantID,
		ChainName: req.ChainName,
		ChainType: strings.ToLower(req.ChainType),
		Enable:    req.Enable,
	}

	switch strings.ToLower(req.ChainType) {
	case ChainTypeChainmaker:
		config.ChainId = req.ChainId
		config.AuthType = req.AuthType
		config.OrgId = req.OrgId
		config.HashType = req.HashType
		config.SignKey = req.SignKey
		config.SignCert = req.SignCert
		config.UserTlsKey = req.UserTlsKey
		config.UserTlsCert = req.UserTlsCert
		config.UserEncKey = req.UserEncKey
		config.UserEncCert = req.UserEncCert
		config.ProxyUrl = req.ProxyUrl
	case ChainTypeEthereum:
		config.EthChainId = req.EthChainId
		config.HttpUrl = req.HttpUrl
		config.WebsocketUrl = req.WebsocketUrl
		config.PrivateKey = req.PrivateKey
		config.GasLimit = req.GasLimit
	case ChainTypeSolana:
		config.SolRpcUrl = req.SolRpcUrl
		config.SolPrivateKey = req.SolPrivateKey
		config.CommitmentLevel = req.CommitmentLevel
		config.SkipPreflight = req.SkipPreflight
		config.MaxRetries = req.MaxRetries
	}

	return config
}

// buildChainNodesFromRequest 从请求体构建节点配置列表
func buildChainNodesFromRequest(chainConfigID uint, nodeReqs []NodeRequest) []*store.TenantChainNode {
	nodes := make([]*store.TenantChainNode, 0, len(nodeReqs))
	for _, n := range nodeReqs {
		connCnt := n.ConnCnt
		if connCnt <= 0 {
			connCnt = 10 // 默认连接数
		}
		nodes = append(nodes, &store.TenantChainNode{
			ChainConfigID: chainConfigID,
			NodeAddr:      n.NodeAddr,
			ConnCnt:       connCnt,
			EnableTls:     n.EnableTls,
			TlsHostName:   n.TlsHostName,
			CaCert:        n.CaCert,
		})
	}
	return nodes
}
