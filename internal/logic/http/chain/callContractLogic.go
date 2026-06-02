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

	// 调用合约（默认 Invoke 类型，超时 10s，同步等待结果）
	start := time.Now()
	txId, result, err := client.CallContract(pb.MethodType_Invoke, req.ContractName, req.Method, kvPairs, 10, true)
	duration := time.Since(start)

	if err != nil {
		// 记录调用日志
		go l.recordCallLog(tenantID, req, "failed", err.Error(), duration)
		return &types.CommonResponse{Code: 500, Message: "invoke contract: " + err.Error()}, nil
	}

	// 记录调用日志
	go l.recordCallLog(tenantID, req, "success", "", duration)

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"tx_id":    txId,
			"result":   result,
			"duration": duration.Milliseconds(),
		},
	}, nil
}

// recordCallLog 异步记录调用日志
func (l *CallContractLogic) recordCallLog(tenantID uint, req *types.CallContractRequest,
	status, errMsg string, duration time.Duration) {

	userID := middleware.GetUserIDFromContext(l.ctx)
	apiKeyID := middleware.GetAPIKeyIDFromContext(l.ctx)

	log := &store.CallLog{
		TenantID:     tenantID,
		UserID:       userID,
		APIKeyID:     apiKeyID,
		ChainName:    req.ChainName,
		Method:       req.Method,
		ContractName: req.ContractName,
		Status:       status,
		ErrorMsg:     errMsg,
		Duration:     duration.Milliseconds(),
	}

	if err := l.svcCtx.Repo.CreateCallLog(context.Background(), log); err != nil {
		logx.Errorf("recordCallLog failed: %v", err)
	}
}
