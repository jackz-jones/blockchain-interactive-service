package event

import (
	"context"
	"encoding/json"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const (
	chainName                = "chain_name"
	internalError            = "internal error"
	msgContractNotOwnedEvent = "contract not owned by current tenant"
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
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	// 从数据库查询当前租户下所有 enable_subscribe=true 的合约配置
	contracts, err := l.svcCtx.Repo.ListEnabledSubscribeContracts(l.ctx, tenantID)
	if err != nil {
		l.Errorf("failed to list enabled subscribe contracts: %v", err)
		return &types.CommonResponse{Code: 500, Message: internalError}, nil
	}

	var items []map[string]interface{}
	for _, contract := range contracts {
		item := map[string]interface{}{
			contractConfigId: contract.ID,
			chainConfigId:    contract.ChainConfigID,
			chainName:        contract.ChainConfig.ChainName,
			chainType:        contract.ChainConfig.ChainType,
			contractName:     contract.ContractName,
			contractAddr:     contract.ContractAddr,
			"status":         "active",
			"created_at":     contract.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		// 解析 extra_conf 返回订阅参数
		if contract.ExtraConf != "" {
			var extraConf map[string]interface{}
			if err := json.Unmarshal([]byte(contract.ExtraConf), &extraConf); err == nil {
				item["extra_conf"] = extraConf
			}
		}
		items = append(items, item)
	}

	if items == nil {
		items = []map[string]interface{}{}
	}

	return &types.CommonResponse{
		Code:    0,
		Message: successStr,
		Data: map[string]interface{}{
			itemsStr: items,
			totalStr: len(items),
		},
	}, nil
}
