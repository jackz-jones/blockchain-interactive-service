package event

import (
	"context"
	"strconv"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const (
	invalidContractConfigId = "invalid contract_config_id"
	contractConfigId        = "contract_config_id"
)

type UnsubscribeByContractLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnsubscribeByContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnsubscribeByContractLogic {
	return &UnsubscribeByContractLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnsubscribeByContractLogic) UnsubscribeByContract(
	contractConfigIDStr string,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	contractConfigID, err := strconv.ParseUint(contractConfigIDStr, 10, 64)
	if err != nil || contractConfigID == 0 {
		return &types.CommonResponse{Code: 400, Message: invalidContractConfigId}, nil
	}

	// 查询合约配置
	contract, err := l.svcCtx.Repo.GetContractConfig(l.ctx, uint(contractConfigID))
	if err != nil {
		l.Errorf("failed to get contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: internalError}, nil
	}
	if contract == nil {
		return &types.CommonResponse{Code: 404, Message: contractConfigNotFound}, nil
	}

	// 校验租户归属
	if contract.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: msgContractNotOwnedEvent}, nil
	}

	// 检查是否已关闭订阅
	if !contract.EnableSubscribe {
		return &types.CommonResponse{Code: 400, Message: "该合约未开启订阅"}, nil
	}

	// 更新 DB 字段：关闭订阅
	contract.EnableSubscribe = false
	if err := l.svcCtx.Repo.UpdateContractConfig(l.ctx, contract); err != nil {
		l.Errorf("failed to update contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: internalError}, nil
	}

	// 停止对应的链监听协程
	l.svcCtx.TenantSDKManager.StopContractSubscription(
		contract.ChainConfigID, contract.ID, tenantID, "")

	// 清理 SubscribeFlag
	flagKey := sdk.SubscribeKeyByID(contract.ChainConfigID, contract.ID)
	sdk.SubscribeFlag.Delete(flagKey)

	l.Infof("subscription disabled for contract %d (chain_config_id=%d, tenant=%d)",
		contract.ID, contract.ChainConfigID, tenantID)

	return &types.CommonResponse{
		Code:    0,
		Message: successStr,
		Data: map[string]interface{}{
			contractConfigId: contract.ID,
			enableSubscribe:  false,
		},
	}, nil
}
