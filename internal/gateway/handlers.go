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

// GetAvailableChainsHandler 获取可用链列表 Handler
func GetAvailableChainsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		chains, err := svcCtx.TenantSDKManager.ListTenantChains(r.Context(), tenantID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list chains: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"chains": chains,
		})
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

// CreateChainConfigRequestBody 创建链配置请求体
type CreateChainConfigRequestBody struct {
	ChainName string `json:"chain_name"`
	ChainType string `json:"chain_type"`
	Enable    bool   `json:"enable"`
	SdkConf   string `json:"sdk_conf"` // JSON 字符串
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

		if req.ChainName == "" || req.ChainType == "" {
			errorResponse(w, http.StatusBadRequest, "chain_name and chain_type are required")
			return
		}

		config := &store.TenantChainConfig{
			TenantID:  tenantID,
			ChainName: req.ChainName,
			ChainType: strings.ToLower(req.ChainType),
			Enable:    req.Enable,
			SdkConf:   req.SdkConf,
		}

		if err := svcCtx.Repo.CreateChainConfig(r.Context(), config); err != nil {
			errorResponse(w, http.StatusInternalServerError, "create chain config: "+err.Error())
			return
		}

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

		successResponse(w, configs)
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

		config := &store.TenantChainConfig{
			TenantID:  tenantID,
			ChainName: req.ChainName,
			ChainType: strings.ToLower(req.ChainType),
			Enable:    req.Enable,
			SdkConf:   req.SdkConf,
		}
		config.ID = uint(id)

		if err := svcCtx.Repo.UpdateChainConfig(r.Context(), config); err != nil {
			errorResponse(w, http.StatusInternalServerError, "update chain config: "+err.Error())
			return
		}

		// 使缓存失效，下次请求时会重新加载
		svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, req.ChainName)

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

		if err := svcCtx.Repo.DeleteChainConfig(r.Context(), uint(id)); err != nil {
			errorResponse(w, http.StatusInternalServerError, "delete chain config: "+err.Error())
			return
		}

		// 使该租户所有缓存失效
		svcCtx.TenantSDKManager.InvalidateAllTenantCache(tenantID)

		successResponse(w, nil)
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
		CreatedAt:    time.Now(),
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
