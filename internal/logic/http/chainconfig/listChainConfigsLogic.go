package chainconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/jackz-jones/blockchain-interactive-service/internal/validator"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListChainConfigsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListChainConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListChainConfigsLogic {
	return &ListChainConfigsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListChainConfigsLogic) ListChainConfigs() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	configs, err := l.svcCtx.Repo.ListChainConfigsByTenant(l.ctx, tenantID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list chain configs: " + err.Error()}, nil
	}

	// 对敏感字段进行脱敏
	maskedConfigs := make([]interface{}, 0, len(configs))
	for _, cfg := range configs {
		maskedConfigs = append(maskedConfigs, validator.MaskChainConfigSensitiveFields(cfg))
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: maskedConfigs}, nil
}
