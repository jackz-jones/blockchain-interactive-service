package apikey

import (
	"context"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAPIKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAPIKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAPIKeyLogic {
	return &CreateAPIKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateAPIKeyLogic) CreateAPIKey(req *types.CreateAPIKeyRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	userID := middleware.GetUserIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &t
	}

	apiKey, err := l.svcCtx.TenantService.CreateAPIKey(l.ctx, &tenant.CreateAPIKeyRequest{
		TenantID:    tenantID,
		UserID:      userID,
		Name:        req.Name,
		Permissions: req.Permissions,
		IPWhitelist: req.IpWhitelist,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "create api key: " + err.Error()}, nil
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"id":  apiKey.ID,
			"key": apiKey.Key,
		},
	}, nil
}
