package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

			// 提前读取请求体，用于审计日志记录合约调用信息
			var bodyBytes []byte
			if r.Body != nil && r.Method != http.MethodGet {
				bodyBytes, _ = io.ReadAll(r.Body)
				// 恢复 r.Body 供后续 handler 读取
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// 包装 ResponseWriter 以捕获状态码和响应体
			wrapped := &bodyResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &strings.Builder{},
				maxBodyBytes:   maxAuditResponseBytes,
			}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// 异步记录审计日志
			go recordHTTPAudit(repo, r, wrapped.statusCode, wrapped.body.String(), duration, bodyBytes)
		})
	}
}

// recordHTTPAudit 记录 HTTP 审计日志
func recordHTTPAudit(repo store.Repository, r *http.Request, statusCode int, responseBody string, duration time.Duration, bodyBytes []byte) {
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
	// 包括：create/update chain_config、update contract_config
	if resourceType == "chain-configs" {
		return
	}
	if (r.Method == http.MethodPut || r.Method == http.MethodPatch) &&
		resourceType == "contract_config" {
		return
	}

	detail := map[string]interface{}{
		"method":      r.Method,
		"path":        r.URL.Path,
		"query":       r.URL.RawQuery,
		"status_code": statusCode,
		"duration":    duration.Milliseconds(),
	}

	// 将非合约调用的请求体也纳入 detail，同时脱敏，避免 private_key/password 等敏感字段直接落库。
	if resourceType != "contract_call" && len(bodyBytes) > 0 {
		maskedBody := maskSensitiveFields(string(bodyBytes))
		// 控制入库长度，避免异常请求体拖油表。
		if len(maskedBody) > maxAuditResponseBytes {
			maskedBody = maskedBody[:maxAuditResponseBytes]
		}
		detail["request"] = maskedBody
	}

	// 合约调用特殊脱敏：detail 不含完整参数，仅记录方法名、合约名、链名、状态、耗时
	if resourceType == "contract_call" {
		// 从请求体中提取合约调用的关键信息
		contractDetail := map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": statusCode,
			"duration":    duration.Milliseconds(),
		}

		// 解析请求体，提取链名、合约名、方法等信息（不对params做记录以避免泄露敏感数据）
		if len(bodyBytes) > 0 {
			var callReq struct {
				ChainName    string            `json:"chain_name"`
				ContractName string            `json:"contract_name"`
				Method       string            `json:"method"`
				MethodType   int               `json:"method_type"`
				Params       map[string]string `json:"params"`
			}
			if json.Unmarshal(bodyBytes, &callReq) == nil {
				if callReq.ChainName != "" {
					contractDetail["chain_name"] = callReq.ChainName
				}
				if callReq.ContractName != "" {
					contractDetail["contract_name"] = callReq.ContractName
				}
				if callReq.Method != "" {
					contractDetail["contract_method"] = callReq.Method
				}
				if callReq.MethodType > 0 {
					methodTypeLabel := "读链(Query)"
					if callReq.MethodType == 1 {
						methodTypeLabel = "写链(Invoke)"
					}
					contractDetail["method_type"] = callReq.MethodType
					contractDetail["method_type_label"] = methodTypeLabel
				}
				// 记录参数键值对（完整键值，业务侧要求保留）
				if len(callReq.Params) > 0 {
					contractDetail["params"] = callReq.Params
				}
			}
		}
		detail = contractDetail
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

// maxAuditResponseBytes 审计缓存响应体的最大字节数（仅用于判断业务码，非透传给客户端）。
// 超出部分不再写入 body 缓冲，避免大响应导致内存放大。
const maxAuditResponseBytes = 64 * 1024

// bodyResponseWriter 包装 ResponseWriter 以捕获状态码和响应体
type bodyResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	body         *strings.Builder
	maxBodyBytes int
}

func (w *bodyResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyResponseWriter) Write(b []byte) (int, error) {
	// 只在未达到上限时缓存响应体，用于后续解析业务码；
	// 无论是否缓存，都必须原样写回底层 ResponseWriter，保证给客户端返回完整内容。
	if w.maxBodyBytes > 0 {
		if remain := w.maxBodyBytes - w.body.Len(); remain > 0 {
			if len(b) <= remain {
				w.body.Write(b)
			} else {
				w.body.Write(b[:remain])
			}
		}
	} else {
		w.body.Write(b)
	}
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

// maskedPlaceholder 脱敏后的占位值
const maskedPlaceholder = "***MASKED***"

// sensitiveFieldSet 需要脱敏的字段名集合（键统一为小写，便于大小写无关匹配）
var sensitiveFieldSet = map[string]struct{}{
	"private_key": {},
	"privatekey":  {},
	"password":    {},
	"secret":      {},
	"api_key":     {},
	"apikey":      {},
	"token":       {},
	"credential":  {},
	"credentials": {},
}

// isSensitiveField 判断字段名是否为敏感字段（大小写无关）
func isSensitiveField(name string) bool {
	_, ok := sensitiveFieldSet[strings.ToLower(name)]
	return ok
}

// maskSensitiveFields 对 JSON 字符串进行结构化脱敏：
// - 解析成 map/array 递归遍历，命中敏感 key 时替换 value；
// - 若解析失败（例如非 JSON），回退到 fallback 的字符串替换实现，尽力保护敏感数据。
func maskSensitiveFields(jsonStr string) string {
	if jsonStr == "" {
		return jsonStr
	}

	trimmed := strings.TrimSpace(jsonStr)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var v interface{}
		if err := json.Unmarshal([]byte(jsonStr), &v); err == nil {
			masked := maskValue(v)
			if out, err := json.Marshal(masked); err == nil {
				return string(out)
			}
		}
	}

	// fallback：非结构化 JSON，退化为逐字段替换（覆盖全部同名字段）
	for field := range sensitiveFieldSet {
		jsonStr = maskFieldValueAll(jsonStr, field)
	}
	return jsonStr
}

// maskValue 递归遍历 JSON 反序列化后的结构，对敏感字段执行脱敏
func maskValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, item := range val {
			if isSensitiveField(k) {
				val[k] = maskedPlaceholder
				continue
			}
			val[k] = maskValue(item)
		}
		return val
	case []interface{}:
		for i, item := range val {
			val[i] = maskValue(item)
		}
		return val
	default:
		return v
	}
}

// maskFieldValueAll fallback：对所有出现的 "field":"..." 均脱敏（不解析 JSON 转义，仅用于非结构化文本兜底）
func maskFieldValueAll(jsonStr, field string) string {
	patterns := []string{
		`"` + field + `":"`,
		`"` + field + `": "`,
	}

	for _, pattern := range patterns {
		var builder strings.Builder
		remaining := jsonStr
		for {
			idx := strings.Index(remaining, pattern)
			if idx == -1 {
				builder.WriteString(remaining)
				break
			}
			valueStart := idx + len(pattern)
			valueEnd := strings.Index(remaining[valueStart:], `"`)
			if valueEnd == -1 {
				builder.WriteString(remaining)
				break
			}
			builder.WriteString(remaining[:valueStart])
			builder.WriteString(maskedPlaceholder)
			remaining = remaining[valueStart+valueEnd:]
		}
		jsonStr = builder.String()
	}
	return jsonStr
}

// MaskSensitiveData 公开的脱敏函数，供其他模块使用
func MaskSensitiveData(data string) string {
	return maskSensitiveFields(data)
}
