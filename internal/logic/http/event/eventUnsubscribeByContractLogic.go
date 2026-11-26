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

func (l *UnsubscribeByContractLogic) UnsubscribeByContract(contractConfigIDStr string) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	contractConfigID, err := strconv.ParseUint(contractConfigIDStr, 10, 64)
	if err != nil || contractConfigID == 0 {
		return &types.CommonResponse{Code: 400, Message: "invalid contract_config_id"}, nil
	}

	// 查询合约配置
	contract, err := l.svcCtx.Repo.GetContractConfig(l.ctx, uint(contractConfigID))
	if err != nil {
		l.Logger.Errorf("failed to get contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: "internal error"}, nil
	}
	if contract == nil {
		return &types.CommonResponse{Code: 404, Message: "contract config not found"}, nil
	}

	// 校验租户归属
	if contract.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "contract not owned by current tenant"}, nil
	}

	// 检查是否已关闭订阅
	if !contract.EnableSubscribe {
		return &types.CommonResponse{Code: 400, Message: "该合约未开启订阅"}, nil
	}

	// 更新 DB 字段：关闭订阅
	contract.EnableSubscribe = false
	if err := l.svcCtx.Repo.UpdateContractConfig(l.ctx, contract); err != nil {
		l.Logger.Errorf("failed to update contract config: %v", err)
		return &types.CommonResponse{Code: 500, Message: "internal error"}, nil
	}

	// 停止对应的链监听协程
	l.svcCtx.TenantSDKManager.StopContractSubscription(
		contract.ChainConfigID, contract.ID, tenantID, "")

	// 清理 SubscribeFlag
	flagKey := sdk.SubscribeKeyByID(contract.ChainConfigID, contract.ID)
	sdk.SubscribeFlag.Delete(flagKey)

	l.Logger.Infof("subscription disabled for contract %d (chain_config_id=%d, tenant=%d)",
		contract.ID, contract.ChainConfigID, tenantID)

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"contract_config_id": contract.ID,
			"enable_subscribe":   false,
		},
	}, nil
}
