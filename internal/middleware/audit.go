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
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
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
func (a *AuditInterceptor) recordGRPCAudit(ctx context.Context, method string, req interface{}, callErr error, duration time.Duration) {
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
		CreatedAt: time.Now(),
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

			// 包装 ResponseWriter 以捕获状态码
			wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			// 异步记录审计日志
			go recordHTTPAudit(repo, r, wrapped.statusCode, duration)
		})
	}
}

// recordHTTPAudit 记录 HTTP 审计日志
func recordHTTPAudit(repo store.Repository, r *http.Request, statusCode int, duration time.Duration) {
	tenantID := GetTenantIDFromHTTP(r)
	userID := GetUserIDFromHTTP(r)

	detail := map[string]interface{}{
		"method":      r.Method,
		"path":        r.URL.Path,
		"query":       r.URL.RawQuery,
		"status_code": statusCode,
		"duration":    duration.Milliseconds(),
	}
	detailBytes, _ := json.Marshal(detail)

	log := &store.AuditLog{
		TenantID:  tenantID,
		UserID:    userID,
		Action:    r.Method + " " + extractResourceFromPath(r.URL.Path),
		Resource:  r.URL.Path,
		Detail:    string(detailBytes),
		IP:        getHTTPClientIP(r),
		UserAgent: r.UserAgent(),
		CreatedAt: time.Now(),
	}

	if err := repo.CreateAuditLog(context.Background(), log); err != nil {
		logx.Errorf("[Audit] record http audit log error: %v", err)
	}
}

// statusResponseWriter 包装 ResponseWriter 以捕获状态码
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
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
