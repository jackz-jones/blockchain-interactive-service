package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSubscriptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSubscriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSubscriptionLogic {
	return &UpdateSubscriptionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSubscriptionLogic) UpdateSubscription(
	req *types.UpdateSubscriptionRequest,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required for write operations"}, nil
	}

	contractConfigID, err := strconv.ParseUint(req.ContractConfigID, 10, 64)
	if err != nil {
		return &types.CommonResponse{Code: 400, Message: "invalid contractConfigId"}, nil
	}

	// 获取合约配置
	contract, err := l.svcCtx.Repo.GetContractConfig(l.ctx, uint(contractConfigID))
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get contract config: " + err.Error()}, nil
	}
	if contract == nil {
		return &types.CommonResponse{Code: 404, Message: "contract config not found"}, nil
	}
	if contract.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "contract config not owned by current tenant"}, nil
	}

	// 解析现有的 extra_conf
	extraConf := map[string]interface{}{}
	if contract.ExtraConf != "" {
		_ = json.Unmarshal([]byte(contract.ExtraConf), &extraConf)
	}

	// 更新订阅开关
	if req.EnableSubscribe != nil {
		contract.EnableSubscribe = *req.EnableSubscribe
	}

	// 更新订阅参数到 extra_conf
	if req.DeployBlockHeight != nil {
		extraConf["DeployBlockHeight"] = *req.DeployBlockHeight
	}
	if req.GetHistoryEventInterval != nil {
		extraConf["GetHistoryEventInterval"] = *req.GetHistoryEventInterval
	}
	if req.GetHistoryEventHeightWindow != nil {
		extraConf["GetHistoryEventHeightWindow"] = *req.GetHistoryEventHeightWindow
	}

	// 序列化 extra_conf
	if extraConfBytes, err := json.Marshal(extraConf); err == nil {
		contract.ExtraConf = string(extraConfBytes)
	}

	if err := l.svcCtx.Repo.UpdateContractConfig(l.ctx, contract); err != nil {
		return &types.CommonResponse{Code: 500, Message: "update subscription: " + err.Error()}, nil
	}

	// 处理订阅状态变更
	chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, contract.ChainConfigID)
	if chainConfig != nil {
		if contract.EnableSubscribe {
			// 开启订阅：中断旧的订阅协程，调度器将基于新配置自动重启
			l.svcCtx.TenantSDKManager.StopContractSubscription(
				contract.ChainConfigID, contract.ID, tenantID, chainConfig.ChainName,
			)
			logx.Infof("[Subscription] restarted subscription after config update: chainConfigID=%d, contractConfigID=%d",
				contract.ChainConfigID, contract.ID)
		} else {
			// 关闭订阅：中断订阅协程
			l.svcCtx.TenantSDKManager.StopContractSubscription(
				contract.ChainConfigID, contract.ID, tenantID, chainConfig.ChainName,
			)
			logx.Infof("[Subscription] stopped subscription: chainConfigID=%d, contractConfigID=%d",
				contract.ChainConfigID, contract.ID)
		}
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: map[string]interface{}{
		"contract_config_id": contract.ID,
		"enable_subscribe":   contract.EnableSubscribe,
		"extra_conf":         fmt.Sprintf("updated with %d params", len(extraConf)),
	}}, nil
}
