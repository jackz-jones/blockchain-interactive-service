package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// CallContractLogic 定义了合约调用逻辑执行对象
type CallContractLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// NewCallContractLogic 初始化合约调用逻辑执行对象
func NewCallContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CallContractLogic {
	return &CallContractLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CallContract 调用指定链上的智能合约
func (l *CallContractLogic) CallContract(in *pb.CallContractRequest) (*pb.TxResponse, error) {

	// 日志通用信息
	fields := map[string]interface{}{
		"requestId":      in.RequestId,
		"chainName":      in.ChainName,
		"contractName":   in.ContractName,
		"contractMethod": in.ContractMethod,
		"kvPairs":        in.KvPairs,
		"methodType":     in.MethodType,
		"withSyncResult": in.WithSyncResult,
		"txTimeout":      in.TxTimeout,
	}
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("receive CallContract request")

	// 获取 SDK 客户端：DB 优先，配置文件回退
	sdkClient, err := l.getSDKClient(in.ChainName, fields)
	if err != nil {
		return l.errorResponse(code.ErrGetSDKClient, err, nil), nil
	}

	// 异步调用上链默认返回交易 pending 状态
	pending := true

	// 不传则默认30秒
	txTimeout := in.TxTimeout
	if in.TxTimeout <= 0 {
		txTimeout = 30
	}
	txId, txData, err := sdkClient.CallContract(in.MethodType, in.ContractName, in.ContractMethod,
		in.KvPairs, txTimeout, in.WithSyncResult)
	fields["txId"] = txId
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrSendTransaction.String())

		// 如果是以太坊交易，同步获取 receipt 超时错误，交易实际已经发送节点，返回错误同时也返回 txId
		if strings.Contains(err.Error(), code.ErrGetTxReceiptTimeoutMsg) {
			return l.errorResponse(code.ErrSendTransaction, err, &pb.TxData{
				ChainName: in.ChainName,
				Content:   txData,
				TxId:      txId,
			}), nil
		}

		return l.errorResponse(code.ErrSendTransaction, err, nil), nil
	}

	// 如果是同步调用完成，则认为交易已经被打包块中
	if in.WithSyncResult {
		pending = false
	}

	// 返回成功信息
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("CallContract success return")
	return l.successResponse(&pb.TxData{
		ChainName: in.ChainName,
		Content:   txData,
		TxId:      txId,
		Pending:   pending,
	}), nil
}

// getSDKClient 获取 SDK 客户端，支持 DB 优先查找，配置文件回退
// 1. 如果 context 中有租户身份（tenantID > 0），先尝试从 DB（TenantSDKManager）查找
// 2. DB 查找失败则回退到配置文件路径（sdkClients sync.Map）
// 3. 未提供租户身份时仅从配置文件查找（向后兼容）
func (l *CallContractLogic) getSDKClient(
	chainName string, fields map[string]interface{},
) (sdk.ChainSdkInterface, error) {
	// 尝试从 context 获取租户 ID（由 gRPC auth interceptor 注入）
	tenantID := middleware.GetTenantID(l.ctx)

	// 如果有租户身份，优先从 DB 查找（DB 配置优先级高于配置文件）
	if tenantID > 0 {
		client, err := l.svcCtx.TenantSDKManager.GetTenantSDKClient(l.ctx, tenantID, chainName)
		if err == nil {
			l.Logger.WithFields(util.ConvertToLogFields(fields)...).
				Infof("got SDK client from DB: tenant=%d, chain=%s", tenantID, chainName)
			return client, nil
		}
		// DB 查找失败，记录日志后回退到配置文件
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).
			Infof("DB lookup failed (tenant=%d, chain=%s): %v, fallback to config file",
				tenantID, chainName, err)
	}

	// 配置文件路径：检查 chainConf 是否存在
	chainConf, exist := l.svcCtx.Config.ChainConfs[chainName]
	if !exist {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotExist.String())
		return nil, fmt.Errorf("%s", code.ErrChainNotExist.String())
	}

	// 如果链未启用，直接返回错误
	if !chainConf.Enable {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotEnable.String())
		return nil, fmt.Errorf("%s", code.ErrChainNotEnable.String())
	}

	// 从配置文件获取 SDK 客户端
	sdkClient, err := sdk.GetSDKClient(l.svcCtx.RootCtx, &l.svcCtx.SDKClients, chainName, l.Logger, chainConf,
		l.svcCtx.Config.Log, l.svcCtx.RedisClient, l.svcCtx.ChainClientFactory)
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrGetSDKClient.String())
		return nil, err
	}

	return sdkClient, nil
}

// errorResponse returns the error response.
func (l *CallContractLogic) errorResponse(code code.RespCode, err error, data *pb.TxData) *pb.TxResponse {
	msg := code.String()
	if err != nil {
		msg = err.Error()
	}

	return &pb.TxResponse{
		Code: int32(code),
		Msg:  msg,
		Data: data,
	}
}

// successResponse returns the success response.
func (l *CallContractLogic) successResponse(data *pb.TxData) *pb.TxResponse {
	return &pb.TxResponse{
		Code: int32(code.Success),
		Msg:  code.Success.String(),
		Data: data,
	}
}
