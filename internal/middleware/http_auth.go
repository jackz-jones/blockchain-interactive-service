package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
)

// HTTP 上下文 key
const (
	HTTPHeaderAPIKey = "X-API-Key"
)

// httpContextKey HTTP 请求上下文 key 类型
type httpContextKey string

const (
	httpKeyTenantID httpContextKey = "http_tenant_id"
	httpKeyUserID   httpContextKey = "http_user_id"
	httpKeyUserRole httpContextKey = "http_user_role"
	httpKeyAPIKeyID httpContextKey = "http_api_key_id"
)

// HTTPAuthMiddleware HTTP API Key 认证中间件
// 若传入非 nil 的 cache，则会：
//  1. 缓存 API Key + Tenant + User 的组合认证结果，命中时不再查 DB
//  2. 对 UpdateAPIKeyLastUsed 做节流，避免每次请求都写 DB
//  3. 对不存在的 key 做短期负缓存，防止暴力扫描打穿 DB
func HTTPAuthMiddleware(repo store.Repository, cache *APIKeyAuthCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 从 Header 中提取 API Key
			apiKeyStr := r.Header.Get(HTTPHeaderAPIKey)
			if apiKeyStr == "" {
				// 也支持 Bearer Token 格式
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					apiKeyStr = strings.TrimPrefix(auth, "Bearer ")
				}
			}

			if apiKeyStr == "" {
				http.Error(w, `{"code":401,"message":"missing api key"}`, http.StatusUnauthorized)
				return
			}

			// 命中缓存：直接使用缓存中的认证要素
			if cache != nil {
				if info, ok := cache.Lookup(apiKeyStr); ok {
					if !isAuthInfoValid(info, r) {
						http.Error(w, `{"code":401,"message":"api key not authorized"}`, http.StatusUnauthorized)
						return
					}
					cache.UpdateLastUsedAsync(repo, info.APIKeyID)
					ctx := injectHTTPAuthCtx(r.Context(), info)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				if cache.LookupNotFound(apiKeyStr) {
					http.Error(w, `{"code":401,"message":"invalid api key"}`, http.StatusUnauthorized)
					return
				}
			}

			// 查询 API Key
			apiKey, err := repo.GetAPIKeyByKey(r.Context(), apiKeyStr)
			if err != nil {
				logx.WithContext(r.Context()).Errorf("query api key error: %v", err)
				http.Error(w, `{"code":500,"message":"internal error"}`, http.StatusInternalServerError)
				return
			}
			if apiKey == nil {
				if cache != nil {
					cache.StoreNotFound(apiKeyStr)
				}
				http.Error(w, `{"code":401,"message":"invalid api key"}`, http.StatusUnauthorized)
				return
			}

			// 检查状态
			if apiKey.Status != "active" {
				http.Error(w, `{"code":401,"message":"api key is revoked"}`, http.StatusUnauthorized)
				return
			}

			// 检查过期
			if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
				http.Error(w, `{"code":401,"message":"api key expired"}`, http.StatusUnauthorized)
				return
			}

			// 检查 IP 白名单
			if apiKey.IPWhitelist != "" {
				clientIP := getHTTPClientIP(r)
				if !isIPAllowed(clientIP, apiKey.IPWhitelist) {
					http.Error(w, `{"code":403,"message":"ip not in whitelist"}`, http.StatusForbidden)
					return
				}
			}

			// 检查租户状态
			t, err := repo.GetTenantByID(r.Context(), apiKey.TenantID)
			if err != nil {
				logx.WithContext(r.Context()).Errorf("query tenant error: %v", err)
				http.Error(w, `{"code":500,"message":"internal error"}`, http.StatusInternalServerError)
				return
			}
			if t == nil || t.Status != store.TenantStatusActive {
				http.Error(w, `{"code":403,"message":"tenant is disabled or suspended"}`, http.StatusForbidden)
				return
			}

			// 查询用户角色
			user, _ := repo.GetUserByID(r.Context(), apiKey.UserID)

			// 组装认证要素
			info := &apiKeyAuthInfo{
				APIKeyID:     apiKey.ID,
				TenantID:     apiKey.TenantID,
				UserID:       apiKey.UserID,
				Status:       apiKey.Status,
				ExpiresAt:    apiKey.ExpiresAt,
				IPWhitelist:  apiKey.IPWhitelist,
				TenantStatus: t.Status,
			}
			if user != nil {
				info.UserRole = user.Role
			}

			// 写入缓存 & 节流更新 last_used
			if cache != nil {
				cache.Store(apiKeyStr, info)
				cache.UpdateLastUsedAsync(repo, apiKey.ID)
			} else {
				// 无缓存时保持原行为：每次都异步写 DB
				go func() {
					_ = repo.UpdateAPIKeyLastUsed(context.Background(), apiKey.ID, time.Now())
				}()
			}

			// 注入认证信息到 context
			ctx := injectHTTPAuthCtx(r.Context(), info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// isAuthInfoValid 校验缓存命中的认证要素是否仍然有效（状态/过期/IP/租户状态）
func isAuthInfoValid(info *apiKeyAuthInfo, r *http.Request) bool {
	if info == nil {
		return false
	}
	if info.Status != "active" {
		return false
	}
	if info.ExpiresAt != nil && info.ExpiresAt.Before(time.Now()) {
		return false
	}
	if info.TenantStatus != store.TenantStatusActive {
		return false
	}
	if info.IPWhitelist != "" {
		if !isIPAllowed(getHTTPClientIP(r), info.IPWhitelist) {
			return false
		}
	}
	return true
}

// injectHTTPAuthCtx 将认证要素注入到 request context
func injectHTTPAuthCtx(parent context.Context, info *apiKeyAuthInfo) context.Context {
	ctx := context.WithValue(parent, httpKeyTenantID, info.TenantID)
	ctx = context.WithValue(ctx, httpKeyAPIKeyID, info.APIKeyID)
	ctx = context.WithValue(ctx, httpKeyUserID, info.UserID)
	if info.UserRole != "" {
		ctx = context.WithValue(ctx, httpKeyUserRole, info.UserRole)
	}
	return ctx
}

// GetTenantIDFromHTTP 从 HTTP 请求 context 中获取租户 ID
func GetTenantIDFromHTTP(r *http.Request) uint {
	if v, ok := r.Context().Value(httpKeyTenantID).(uint); ok {
		return v
	}
	return 0
}

// GetUserIDFromHTTP 从 HTTP 请求 context 中获取用户 ID
func GetUserIDFromHTTP(r *http.Request) uint {
	if v, ok := r.Context().Value(httpKeyUserID).(uint); ok {
		return v
	}
	return 0
}

// GetUserRoleFromHTTP 从 HTTP 请求 context 中获取用户角色
func GetUserRoleFromHTTP(r *http.Request) store.UserRole {
	if v, ok := r.Context().Value(httpKeyUserRole).(store.UserRole); ok {
		return v
	}
	return ""
}

// GetAPIKeyIDFromHTTP 从 HTTP 请求 context 中获取 API Key ID
func GetAPIKeyIDFromHTTP(r *http.Request) uint {
	if v, ok := r.Context().Value(httpKeyAPIKeyID).(uint); ok {
		return v
	}
	return 0
}

// GetTenantIDFromContext 从 context 中获取租户 ID（HTTP 中间件注入）
func GetTenantIDFromContext(ctx context.Context) uint {
	if v, ok := ctx.Value(httpKeyTenantID).(uint); ok {
		return v
	}
	return 0
}

// GetUserIDFromContext 从 context 中获取用户 ID（HTTP 中间件注入）
func GetUserIDFromContext(ctx context.Context) uint {
	if v, ok := ctx.Value(httpKeyUserID).(uint); ok {
		return v
	}
	return 0
}

// GetUserRoleFromContext 从 context 中获取用户角色（HTTP 中间件注入）
func GetUserRoleFromContext(ctx context.Context) store.UserRole {
	if v, ok := ctx.Value(httpKeyUserRole).(store.UserRole); ok {
		return v
	}
	return ""
}

// GetAPIKeyIDFromContext 从 context 中获取 API Key ID（HTTP 中间件注入）
func GetAPIKeyIDFromContext(ctx context.Context) uint {
	if v, ok := ctx.Value(httpKeyAPIKeyID).(uint); ok {
		return v
	}
	return 0
}

// getHTTPClientIP 从 HTTP 请求中获取客户端 IP
func getHTTPClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
