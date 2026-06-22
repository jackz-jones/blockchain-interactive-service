package event

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSubscriptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSubscriptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSubscriptionsLogic {
	return &ListSubscriptionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSubscriptionsLogic) ListSubscriptions() (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	// 从数据库查询当前租户下所有 enable_subscribe=true 的合约配置
	contracts, err := l.svcCtx.Repo.ListEnabledSubscribeContracts(l.ctx, tenantID)
	if err != nil {
		l.Logger.Errorf("failed to list enabled subscribe contracts: %v", err)
		return &types.CommonResponse{Code: 500, Message: "internal error"}, nil
	}

	var items []map[string]interface{}
	for _, contract := range contracts {
		items = append(items, map[string]interface{}{
			"contract_config_id": contract.ID,
			"chain_config_id":    contract.ChainConfigID,
			"chain_name":         contract.ChainConfig.ChainName,
			"contract_name":      contract.ContractName,
			"contract_addr":      contract.ContractAddr,
			"status":             "active",
			"created_at":         contract.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"items": items,
			"total": len(items),
		},
	}, nil
}