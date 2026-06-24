package chain

import (
	"context"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CallContractLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCallContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CallContractLogic {
	return &CallContractLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CallContractLogic) CallContract(req *types.CallContractRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	if req.ChainName == "" || req.ContractName == "" || req.Method == "" {
		return &types.CommonResponse{Code: 400, Message: "chain_name, contract_name and method are required"}, nil
	}

	// 获取租户级 SDK 客户端
	client, err := l.svcCtx.TenantSDKManager.GetTenantSDKClient(l.ctx, tenantID, req.ChainName)
	if err != nil {
		return &types.CommonResponse{Code: 400, Message: "get sdk client: " + err.Error()}, nil
	}

	// 构建参数
	var kvPairs []*pb.KeyValuePair
	for k, v := range req.Params {
		kvPairs = append(kvPairs, &pb.KeyValuePair{Key: k, Value: []byte(v)})
	}

	// 确定调用类型：默认 Invoke（写链），如果指定了 method_type=2 则为 Query（读链）
	methodType := pb.MethodType_Invoke
	if req.MethodType == 2 {
		methodType = pb.MethodType_Query
	}

	// 确定超时时间：默认 10 秒
	txTimeout := req.TxTimeout
	if txTimeout <= 0 {
		txTimeout = 10
	}

	// 调用合约：默认异步（withSyncResult=false），只有明确指定 sync=true 时才同步等待
	start := time.Now()
	txId, result, err := client.CallContract(methodType, req.ContractName, req.Method, kvPairs, txTimeout, req.Sync, req.GasLimit)
	duration := time.Since(start)

	if err != nil {
		// 记录调用日志
		go l.recordCallLog(tenantID, req, txId, "failed", err.Error(), duration)
		return &types.CommonResponse{Code: 500, Message: "invoke contract: " + err.Error()}, nil
	}

	// 记录调用日志
	go l.recordCallLog(tenantID, req, txId, "success", "", duration)

	// 写链异步调用时 pending=true（交易已提交但未确认），同步调用完成后 pending=false
	pending := methodType == pb.MethodType_Invoke && !req.Sync

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"tx_id":    txId,
			"result":   result,
			"pending":  pending,
			"duration": duration.Milliseconds(),
		},
	}, nil
}

// recordCallLog 异步记录调用日志
func (l *CallContractLogic) recordCallLog(tenantID uint, req *types.CallContractRequest,
	txId, status, errMsg string, duration time.Duration) {

	userID := middleware.GetUserIDFromContext(l.ctx)
	apiKeyID := middleware.GetAPIKeyIDFromContext(l.ctx)

	// 从链配置中获取链类型
	var chainType string
	if chainConfig, err := l.svcCtx.Repo.GetChainConfig(context.Background(), tenantID, req.ChainName); err == nil && chainConfig != nil {
		chainType = chainConfig.ChainType
	}

	log := &store.CallLog{
		TenantID:     tenantID,
		UserID:       userID,
		APIKeyID:     apiKeyID,
		ChainName:    req.ChainName,
		ChainType:    chainType,
		TxId:         txId,
		Method:       req.Method,
		MethodType:   store.MethodType(req.MethodType),
		ContractName: req.ContractName,
		Status:       status,
		ErrorMsg:     errMsg,
		Duration:     duration.Milliseconds(),
	}

	if err := l.svcCtx.Repo.CreateCallLog(context.Background(), log); err != nil {
		logx.Errorf("recordCallLog failed: %v", err)
	}
}
