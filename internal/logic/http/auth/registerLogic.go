package auth

import (
	"context"
	"errors"

	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Register 公开注册接口：创建租户 + 默认管理员 + 初始 API Key
func (l *RegisterLogic) Register(req *types.RegisterRequest) (resp interface{}, err error) {
	if req.Name == "" || req.Email == "" || req.Password == "" {
		return nil, &types.BizError{Code: 400, Message: "name, email and password are required"}
	}

	result, err := l.svcCtx.TenantService.CreateTenant(l.ctx, &tenant.CreateTenantRequest{
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		Plan:     "free",
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, tenant.ErrTenantExists) {
			// 租户已存在，尝试恢复缺失的资源（API Key 和 Quota）
			recoverResult, recoverErr := l.svcCtx.TenantService.EnsureTenantResources(l.ctx, req.Name, "free", req.Password)
			if recoverErr != nil {
				logx.Errorf("failed to recover tenant resources: %v", recoverErr)
				return nil, &types.BizError{Code: 409, Message: "tenant already exists"}
			}
			return map[string]interface{}{
				"tenant_id":   recoverResult.Tenant.ID,
				"tenant_name": recoverResult.Tenant.Name,
				"api_key":     recoverResult.APIKey.Key,
				"username":    req.Name + "_admin",
				"recovered":   true,
			}, nil
		}
		return nil, &types.BizError{Code: 500, Message: "create tenant: " + err.Error()}
	}

	return map[string]interface{}{
		"tenant_id":   result.Tenant.ID,
		"tenant_name": result.Tenant.Name,
		"api_key":     result.APIKey.Key,
		"username":    result.Admin.Username,
	}, nil
}
