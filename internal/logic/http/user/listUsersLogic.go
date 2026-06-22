package user

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUsersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListUsersLogic) ListUsers(req *types.ListUsersRequest) (resp *types.CommonResponse, err error) {
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

	users, total, err := l.svcCtx.Repo.ListUsersByTenant(l.ctx, tenantID, (page-1)*pageSize, pageSize)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list users: " + err.Error()}, nil
	}

	// 填充 TenantName 和 LastLogin 虚拟字段
	for _, user := range users {
		// 填充 TenantName：通过关联查询 Tenant
		if tenant, err := l.svcCtx.Repo.GetTenantByID(l.ctx, user.TenantID); err == nil && tenant != nil {
			user.TenantName = tenant.Name
		}
		// 填充 LastLogin：取用户最近一条 CallLog 的创建时间作为最后登录时间
		logs, _, err := l.svcCtx.Repo.ListCallLogs(l.ctx, store.CallLogFilter{
			TenantID: tenantID,
			UserID:   user.ID,
		}, 0, 1)
		if err == nil && len(logs) > 0 {
			user.LastLogin = &logs[0].Model.CreatedAt
		}
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total": total,
			"page":  page,
			"items": users,
		},
	}, nil
}
