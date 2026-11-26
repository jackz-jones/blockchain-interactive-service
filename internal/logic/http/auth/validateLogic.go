package auth

import (
	"context"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateLogic {
	return &ValidateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Validate 公开验证接口：验证 API Key 有效性并返回租户信息
func (l *ValidateLogic) Validate(req *types.ValidateAPIKeyRequest) (resp interface{}, err error) {
	if req.APIKey == "" {
		return nil, &types.BizError{Code: 400, Message: "api_key is required"}
	}

	// 查询 API Key
	apiKey, err := l.svcCtx.Repo.GetAPIKeyByKey(l.ctx, req.APIKey)
	if err != nil {
		l.Logger.Errorf("query api key error: %v", err)
		return nil, &types.BizError{Code: 500, Message: "internal error"}
	}
	if apiKey == nil {
		return nil, &types.BizError{Code: 401, Message: "invalid api key"}
	}

	// 检查状态
	if apiKey.Status != "active" {
		return nil, &types.BizError{Code: 401, Message: "api key is revoked"}
	}

	// 检查过期
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, &types.BizError{Code: 401, Message: "api key expired"}
	}

	// 查询租户信息
	t, err := l.svcCtx.Repo.GetTenantByID(l.ctx, apiKey.TenantID)
	if err != nil {
		return nil, &types.BizError{Code: 500, Message: "internal error"}
	}
	if t == nil {
		return nil, &types.BizError{Code: 404, Message: "tenant not found"}
	}

	return map[string]interface{}{
		"tenant_id":   t.ID,
		"tenant_name": t.Name,
		"role":        "admin",
		"plan":        t.Plan,
	}, nil
}
