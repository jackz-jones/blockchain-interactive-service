package middleware

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RBACInterceptor 基于角色的权限控制拦截器
type RBACInterceptor struct {
	// methodPermissions 方法 -> 允许的最低角色列表
	methodPermissions map[string][]store.UserRole
}

// NewRBACInterceptor 创建 RBAC 拦截器
func NewRBACInterceptor() *RBACInterceptor {
	return &RBACInterceptor{
		methodPermissions: defaultMethodPermissions(),
	}
}

// Unary 一元 RPC 权限校验拦截器
func (r *RBACInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 跳过不需要权限检查的接口
		if shouldSkipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		if err := r.checkPermission(ctx, info.FullMethod); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// Stream 流式 RPC 权限校验拦截器
func (r *RBACInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if shouldSkipAuth(info.FullMethod) {
			return handler(srv, ss)
		}

		if err := r.checkPermission(ss.Context(), info.FullMethod); err != nil {
			return err
		}

		return handler(srv, ss)
	}
}

// checkPermission 检查当前用户是否有权限调用该方法
func (r *RBACInterceptor) checkPermission(ctx context.Context, fullMethod string) error {
	role := GetUserRole(ctx)
	if role == "" {
		// 如果没有角色信息（可能是认证被跳过的情况），放行
		return nil
	}

	// 查找方法所需的角色
	allowedRoles, exists := r.methodPermissions[fullMethod]
	if !exists {
		// 未配置权限的方法，默认所有已认证用户可访问
		return nil
	}

	// 检查用户角色是否在允许列表中
	for _, allowed := range allowedRoles {
		if role == allowed {
			return nil
		}
	}

	return status.Errorf(codes.PermissionDenied,
		"role '%s' does not have permission to access %s", role, fullMethod)
}

// defaultMethodPermissions 默认的方法权限配置
// 所有方法默认允许 admin 和 developer 访问，readonly 只能查询
func defaultMethodPermissions() map[string][]store.UserRole {
	return map[string][]store.UserRole{
		// 合约调用 - 需要 admin 或 developer 角色
		"/pb.ChainInteractive/CallContract": {store.UserRoleAdmin, store.UserRoleDeveloper},

		// 查询类接口 - 所有角色都可以访问
		"/pb.ChainInteractive/GetTxByTxId":                       {store.UserRoleAdmin, store.UserRoleDeveloper, store.UserRoleReadonly},
		"/pb.ChainInteractive/GetAvailableChainAndContractNames": {store.UserRoleAdmin, store.UserRoleDeveloper, store.UserRoleReadonly},
	}
}
