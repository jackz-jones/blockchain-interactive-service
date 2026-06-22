package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// AuditInterceptor 审计日志 gRPC 拦截器
type AuditInterceptor struct {
	repo store.Repository
}

// NewAuditInterceptor 创建审计日志拦截器
func NewAuditInterceptor(repo store.Repository) *AuditInterceptor {
	return &AuditInterceptor{repo: repo}
}

// Unary 一元 RPC 审计日志拦截器
func (a *AuditInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		if shouldSkipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		// 异步记录审计日志
		go a.recordGRPCAudit(ctx, info.FullMethod, req, err, duration)

		return resp, err
	}
}

// recordGRPCAudit 记录 gRPC 审计日志
func (a *AuditInterceptor) recordGRPCAudit(
	ctx context.Context, method string, req interface{},
	callErr error, duration time.Duration,
) {
	tenantID := GetTenantID(ctx)
	userID := GetUserID(ctx)

	// 构建详情
	detail := map[string]interface{}{
		"method":   method,
		"duration": duration.Milliseconds(),
	}
	if callErr != nil {
		detail["error"] = callErr.Error()
	}

	// 脱敏处理请求参数
	reqJSON, _ := json.Marshal(req)
	detail["request"] = maskSensitiveFields(string(reqJSON))

	detailBytes, _ := json.Marshal(detail)

	// 获取客户端 IP
	ip := ""
	if p, ok := peer.FromContext(ctx); ok {
		ip = p.Addr.String()
	}

	// 获取 User-Agent
	userAgent := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ua := md.Get("user-agent"); len(ua) > 0 {
			userAgent = ua[0]
		}
	}

	log := &store.AuditLog{
		TenantID:  tenantID,
		UserID:    userID,
		Action:    extractAction(method),
		Resource:  method,
		Detail:    string(detailBytes),
		IP:        ip,
		UserAgent: userAgent,
	}

	if err := a.repo.CreateAuditLog(context.Background(), log); err != nil {
		logx.Errorf("[Audit] record grpc audit log error: %v", err)
	}
}

// HTTPAuditMiddleware HTTP 审计日志中间件
func HTTPAuditMiddleware(repo store.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 ResponseWriter 以捕获状态码和响应体
			wrapped := &bodyResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &strings.Builder{},
			}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// 异步记录审计日志
			go recordHTTPAudit(repo, r, wrapped.statusCode, wrapped.body.String(), duration)
		})
	}
}

// recordHTTPAudit 记录 HTTP 审计日志
func recordHTTPAudit(repo store.Repository, r *http.Request, statusCode int, responseBody string, duration time.Duration) {
	// 只审计写操作（非 GET 请求）
	if r.Method == http.MethodGet {
		return
	}

	// 只记录成功的操作（2xx），失败的请求不记录审计日志
	if statusCode < 200 || statusCode >= 300 {
		return
	}

	// 检查业务逻辑返回的 code 是否为0（成功），非0表示业务失败，不记录审计日志
	if responseBody != "" {
		var bizResp struct {
			Code int `json:"code"`
		}
		if json.Unmarshal([]byte(responseBody), &bizResp) == nil && bizResp.Code != 0 {
			return
		}
	}

	tenantID := GetTenantIDFromHTTP(r)
	userID := GetUserIDFromHTTP(r)

	// 从 URL 路径提取 action、resource_type、resource_id
	action, resourceType, resourceID := extractAuditInfoFromRequest(r)

	// 对于已在业务逻辑中主动创建审计日志的操作，中间件不再重复记录
	// 包括：update chain_config、update contract_config
	if (r.Method == http.MethodPut || r.Method == http.MethodPatch) &&
		(resourceType == "chain-configs" || resourceType == "contract_config") {
		return
	}

	detail := map[string]interface{}{
		"method":      r.Method,
		"path":        r.URL.Path,
		"query":       r.URL.RawQuery,
		"status_code": statusCode,
		"duration":    duration.Milliseconds(),
	}

	// 合约调用特殊脱敏：detail 不含完整参数，仅记录方法名、合约名、链名、状态、耗时
	if resourceType == "contract_call" {
		detail = map[string]interface{}{
			"method":        r.Method,
			"path":          r.URL.Path,
			"status_code":   statusCode,
			"duration":      duration.Milliseconds(),
			"contract_call": true,
		}
	}

	detailBytes, _ := json.Marshal(detail)

	log := &store.AuditLog{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     action,
		Resource:   resourceType,
		ResourceID: resourceID,
		Detail:     string(detailBytes),
		IP:         getHTTPClientIP(r),
		UserAgent:  r.UserAgent(),
	}

	if err := repo.CreateAuditLog(context.Background(), log); err != nil {
		logx.Errorf("[Audit] record http audit log error: %v", err)
	}
}

// extractAuditInfoFromRequest 从 HTTP 请求中提取审计信息
// 返回 action, resource_type, resource_id
func extractAuditInfoFromRequest(r *http.Request) (string, string, string) {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// 默认值
	action := strings.ToLower(r.Method)
	resourceType := ""
	resourceID := ""

	// 路径格式: /api/v1/{resource_type}[/{resource_id}]
	// 也支持嵌套资源: /api/v1/{parent_type}/{parent_id}/{child_type}[/{child_id}]
	if len(parts) >= 3 {
		resourceType = parts[2]
	}
	if len(parts) >= 4 {
		resourceID = parts[3]
	}

	// 处理嵌套资源路径，如 /api/v1/chain-configs/:id/contracts/:contractId
	// 将路径识别为 resourceType=contract_config, resourceID=末尾的 ID
	if len(parts) >= 6 {
		// 嵌套资源格式: /api/v1/{parent_type}/{parent_id}/{child_type}/{child_id}
		parentType := parts[2]
		childType := parts[4]
		childID := parts[5]

		// 合并父子资源类型，如 chain-configs/contracts → contract_config
		if parentType == "chain-configs" && childType == "contracts" {
			resourceType = "contract_config"
			resourceID = childID
		} else if parentType == "chain-configs" && childType == "test-connection" {
			resourceType = "chain_config_test"
			resourceID = parts[3] // parent_id 即 chain_config_id
		} else {
			// 通用嵌套资源处理
			resourceType = parentType + "_" + childType
			resourceID = childID
		}
	}

	// 特殊处理合约调用：POST /api/v1/contract/call
	if len(parts) >= 3 && parts[2] == "contract" && r.Method == http.MethodPost {
		// 检查是否是 /api/v1/contract/call
		if len(parts) >= 4 && parts[3] == "call" {
			resourceType = "contract_call"
			action = "call"
			resourceID = ""
		}
	}

	// 统一 action 为 create/update/delete/call
	switch action {
	case "post":
		action = "create"
	case "put", "patch":
		action = "update"
	case "delete":
		action = "delete"
	}

	return action, resourceType, resourceID
}

// bodyResponseWriter 包装 ResponseWriter 以捕获状态码和响应体
type bodyResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *strings.Builder
}

func (w *bodyResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// extractAction 从 gRPC 方法路径中提取操作名
func extractAction(method string) string {
	parts := strings.Split(method, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return method
}

// extractResourceFromPath 从 URL 路径中提取资源类型
func extractResourceFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 {
		return parts[2] // /api/v1/{resource}
	}
	return path
}

// ========== 敏感数据脱敏 ==========

// sensitiveFields 需要脱敏的字段名
var sensitiveFields = []string{
	"private_key", "privateKey", "password", "secret",
	"api_key", "apiKey", "token", "credential",
}

// maskSensitiveFields 对 JSON 字符串中的敏感字段进行脱敏
func maskSensitiveFields(jsonStr string) string {
	for _, field := range sensitiveFields {
		// 简单的字符串替换脱敏（生产环境应使用更精确的 JSON 解析）
		jsonStr = maskFieldValue(jsonStr, field)
	}
	return jsonStr
}

// maskFieldValue 脱敏单个字段
func maskFieldValue(jsonStr, field string) string {
	// 匹配 "field":"value" 或 "field": "value" 模式
	patterns := []string{
		`"` + field + `":"`,
		`"` + field + `": "`,
	}

	for _, pattern := range patterns {
		idx := strings.Index(jsonStr, pattern)
		if idx == -1 {
			continue
		}

		valueStart := idx + len(pattern)
		valueEnd := strings.Index(jsonStr[valueStart:], `"`)
		if valueEnd == -1 {
			continue
		}

		masked := jsonStr[:valueStart] + "***MASKED***" + jsonStr[valueStart+valueEnd:]
		jsonStr = masked
	}

	return jsonStr
}

// MaskSensitiveData 公开的脱敏函数，供其他模块使用
func MaskSensitiveData(data string) string {
	return maskSensitiveFields(data)
}
