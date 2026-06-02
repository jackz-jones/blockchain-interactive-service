package tenant

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTenantLogic {
	return &CreateTenantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateTenantLogic) CreateTenant(req *types.CreateTenantRequest) (resp *types.CommonResponse, err error) {
	if req.Name == "" || req.Email == "" || req.Password == "" {
		return &types.CommonResponse{Code: 400, Message: "name, email and password are required"}, nil
	}

	result, err := l.svcCtx.TenantService.CreateTenant(l.ctx, &tenant.CreateTenantRequest{
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		Plan:     req.Plan,
		Password: req.Password,
	})
	if err != nil {
		if err == tenant.ErrTenantExists {
			return &types.CommonResponse{Code: 409, Message: "tenant already exists"}, nil
		}
		return &types.CommonResponse{Code: 500, Message: "create tenant: " + err.Error()}, nil
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"tenant_id": result.Tenant.ID,
			"api_key":   result.APIKey.Key,
			"username":  result.Admin.Username,
		},
	}, nil
}
