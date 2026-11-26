package dashboard

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const ()

type ListBillsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListBillsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBillsLogic {
	return &ListBillsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListBillsLogic) ListBills(req *types.ListBillsRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	bills, total, err := l.svcCtx.Repo.ListBillsByTenant(l.ctx, tenantID, req.BillType, (page-1)*pageSize, pageSize)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list bills: " + err.Error()}, nil
	}

	return &types.CommonResponse{
		Code:    0,
		Message: successStr,
		Data: map[string]interface{}{
			totalStr: total,
			pageStr:  page,
			itemsStr: bills,
		},
	}, nil
}
