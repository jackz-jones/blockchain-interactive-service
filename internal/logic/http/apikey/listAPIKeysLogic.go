package apikey

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAPIKeysLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAPIKeysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAPIKeysLogic {
	return &ListAPIKeysLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAPIKeysLogic) ListAPIKeys(req *types.ListAPIKeysRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	keys, total, err := l.svcCtx.Repo.ListAPIKeysByTenant(l.ctx, tenantID, (page-1)*pageSize, pageSize)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list api keys: " + err.Error()}, nil
	}

	// 填充 KeyMasked 虚拟字段（Key 脱敏显示：前缀+"*********"+后4位）
	for _, key := range keys {
		if len(key.Key) >= 8 {
			prefix := key.Key[:8]
			suffix := key.Key[len(key.Key)-4:]
			key.KeyMasked = prefix + "*********" + suffix
		} else if len(key.Key) > 0 {
			key.KeyMasked = key.Key + "*********"
		} else {
			key.KeyMasked = "-"
		}
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total": total,
			"items": keys,
		},
	}, nil
}
