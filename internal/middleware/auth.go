package middleware

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// 上下文 key 类型
type contextKey string

const (
	// ContextKeyTenantID 租户 ID
	ContextKeyTenantID contextKey = "tenant_id"
	// ContextKeyUserID 用户 ID
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyUserRole 用户角色
	ContextKeyUserRole contextKey = "user_role"
	// ContextKeyAPIKeyID API Key ID
	ContextKeyAPIKeyID contextKey = "api_key_id"

	// MetadataKeyAPIKey 请求 metadata 中的 API Key 字段名
	MetadataKeyAPIKey = "x-api-key"

	// statusActive API Key 活跃状态
	statusActive = "active"
)

// AuthInterceptor API Key 认证拦截器
type AuthInterceptor struct {
	repo  store.Repository
	cache *APIKeyAuthCache
}

// NewAuthInterceptor 创建认证拦截器
func NewAuthInterceptor(repo store.Repository) *AuthInterceptor {
	return &AuthInterceptor{repo: repo}
}

// NewAuthInterceptorWithCache 创建启用缓存的认证拦截器
func NewAuthInterceptorWithCache(repo store.Repository, cache *APIKeyAuthCache) *AuthInterceptor {
	return &AuthInterceptor{repo: repo, cache: cache}
}

// Unary 一元 RPC 认证拦截器
func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		// 跳过健康检查等内部接口
		if shouldSkipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		newCtx, err := a.authenticate(ctx)
		if err != nil {
			return nil, err
		}

		return handler(newCtx, req)
	}
}

// Stream 流式 RPC 认证拦截器
func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if shouldSkipAuth(info.FullMethod) {
			return handler(srv, ss)
		}

		newCtx, err := a.authenticate(ss.Context())
		if err != nil {
			return err
		}

		wrapped := &wrappedServerStream{ServerStream: ss, ctx: newCtx}
		return handler(srv, wrapped)
	}
}

// authenticate 执行认证逻辑
func (a *AuthInterceptor) authenticate(ctx context.Context) (context.Context, error) {
	// 从 metadata 中提取 API Key
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	apiKeyValues := md.Get(MetadataKeyAPIKey)
	if len(apiKeyValues) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing api key")
	}

	apiKeyStr := apiKeyValues[0]
	if apiKeyStr == "" {
		return nil, status.Error(codes.Unauthenticated, "empty api key")
	}

	// 命中缓存路径
	if a.cache != nil {
		if info, ok := a.cache.Lookup(apiKeyStr); ok {
			if err := checkAuthInfoGRPC(info, ctx); err != nil {
				return nil, err
			}
			a.cache.UpdateLastUsedAsync(a.repo, info.APIKeyID)
			return injectGRPCAuthCtx(ctx, info), nil
		}
		if a.cache.LookupNotFound(apiKeyStr) {
			return nil, status.Error(codes.Unauthenticated, "invalid api key")
		}
	}

	// 查询 API Key
	apiKey, err := a.repo.GetAPIKeyByKey(ctx, apiKeyStr)
	if err != nil {
		logx.WithContext(ctx).Errorf("query api key error: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	if apiKey == nil {
		if a.cache != nil {
			a.cache.StoreNotFound(apiKeyStr)
		}
		return nil, status.Error(codes.Unauthenticated, "invalid api key")
	}

	// 检查 API Key 状态
	if apiKey.Status != statusActive {
		return nil, status.Error(codes.Unauthenticated, "api key is revoked")
	}

	// 检查 API Key 是否过期
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "api key expired")
	}

	// 检查 IP 白名单
	if apiKey.IPWhitelist != "" {
		clientIP := getClientIP(ctx)
		if !isIPAllowed(clientIP, apiKey.IPWhitelist) {
			return nil, status.Error(codes.PermissionDenied, "ip not in whitelist")
		}
	}

	// 检查租户状态
	tenant, err := a.repo.GetTenantByID(ctx, apiKey.TenantID)
	if err != nil {
		logx.WithContext(ctx).Errorf("query tenant error: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	if tenant == nil || tenant.Status != store.TenantStatusActive {
		return nil, status.Error(codes.PermissionDenied, "tenant is disabled or suspended")
	}

	// 查询用户角色
	user, err := a.repo.GetUserByID(ctx, apiKey.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("query user error: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}

	// 组装认证要素
	info := &apiKeyAuthInfo{
		APIKeyID:     apiKey.ID,
		TenantID:     apiKey.TenantID,
		UserID:       apiKey.UserID,
		Status:       apiKey.Status,
		ExpiresAt:    apiKey.ExpiresAt,
		IPWhitelist:  apiKey.IPWhitelist,
		TenantStatus: tenant.Status,
	}
	if user != nil {
		info.UserRole = user.Role
	}

	// 写缓存并节流更新 last_used
	if a.cache != nil {
		a.cache.Store(apiKeyStr, info)
		a.cache.UpdateLastUsedAsync(a.repo, apiKey.ID)
	} else {
		go func() {
			_ = a.repo.UpdateAPIKeyLastUsed(context.Background(), apiKey.ID, time.Now())
		}()
	}

	return injectGRPCAuthCtx(ctx, info), nil
}

// checkAuthInfoGRPC 校验缓存命中的认证要素是否仍有效
func checkAuthInfoGRPC(info *apiKeyAuthInfo, ctx context.Context) error {
	if info == nil || info.Status != statusActive {
		return status.Error(codes.Unauthenticated, "api key is revoked")
	}
	if info.ExpiresAt != nil && info.ExpiresAt.Before(time.Now()) {
		return status.Error(codes.Unauthenticated, "api key expired")
	}
	if info.TenantStatus != store.TenantStatusActive {
		return status.Error(codes.PermissionDenied, "tenant is disabled or suspended")
	}
	if info.IPWhitelist != "" {
		if !isIPAllowed(getClientIP(ctx), info.IPWhitelist) {
			return status.Error(codes.PermissionDenied, "ip not in whitelist")
		}
	}
	return nil
}

// injectGRPCAuthCtx 将认证要素注入到 gRPC context
func injectGRPCAuthCtx(parent context.Context, info *apiKeyAuthInfo) context.Context {
	ctx := context.WithValue(parent, ContextKeyTenantID, info.TenantID)
	ctx = context.WithValue(ctx, ContextKeyAPIKeyID, info.APIKeyID)
	ctx = context.WithValue(ctx, ContextKeyUserID, info.UserID)
	if info.UserRole != "" {
		ctx = context.WithValue(ctx, ContextKeyUserRole, info.UserRole)
	}
	return ctx
}

// shouldSkipAuth 判断是否跳过认证（健康检查等）
func shouldSkipAuth(fullMethod string) bool {
	skipMethods := []string{
		"/grpc.health.v1.Health/",
		"/grpc.reflection.v1alpha.ServerReflection/",
	}
	for _, m := range skipMethods {
		if strings.HasPrefix(fullMethod, m) {
			return true
		}
	}
	return false
}

// getClientIP 从 context 中获取客户端 IP
func getClientIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}
	if addr, ok := p.Addr.(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	// 尝试从地址字符串中解析
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}

// isIPAllowed 检查 IP 是否在白名单中
func isIPAllowed(clientIP, whitelist string) bool {
	if clientIP == "" {
		return false
	}
	ips := strings.Split(whitelist, ",")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		// 支持 CIDR 格式
		if strings.Contains(ip, "/") {
			_, ipNet, err := net.ParseCIDR(ip)
			if err != nil {
				continue
			}
			if ipNet.Contains(net.ParseIP(clientIP)) {
				return true
			}
		} else if ip == clientIP {
			return true
		}
	}
	return false
}

// wrappedServerStream 包装 ServerStream 以注入新的 context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// ---- Context 辅助函数 ----

// GetTenantID 从 context 中获取租户 ID
func GetTenantID(ctx context.Context) uint {
	if v, ok := ctx.Value(ContextKeyTenantID).(uint); ok {
		return v
	}
	return 0
}

// GetUserID 从 context 中获取用户 ID
func GetUserID(ctx context.Context) uint {
	if v, ok := ctx.Value(ContextKeyUserID).(uint); ok {
		return v
	}
	return 0
}

// GetUserRole 从 context 中获取用户角色
func GetUserRole(ctx context.Context) store.UserRole {
	if v, ok := ctx.Value(ContextKeyUserRole).(store.UserRole); ok {
		return v
	}
	return ""
}

// GetAPIKeyID 从 context 中获取 API Key ID
func GetAPIKeyID(ctx context.Context) uint {
	if v, ok := ctx.Value(ContextKeyAPIKeyID).(uint); ok {
		return v
	}
	return 0
}
