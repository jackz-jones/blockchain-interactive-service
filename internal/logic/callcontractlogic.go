package logic

import (
	"context"
	"strings"

	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
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

// CallContract 请求本地链合约，给提单平台用的
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

	// 检查 chainConf 是否存在
	chainConf, exist := l.svcCtx.Config.ChainConfs[in.ChainName]
	if !exist {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotExist.String())
		return l.errorResponse(code.ErrChainNotExist, nil, nil), nil
	}

	// 如果链未启用，直接返回错误
	if !chainConf.Enable {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotEnable.String())
		return l.errorResponse(code.ErrChainNotEnable, nil, nil), nil
	}

	// 获取sdk客户端
	sdkClient, err := sdk.GetSDKClient(&l.svcCtx.SDKClients, in.ChainName, l.Logger, chainConf,
		l.svcCtx.Config.Log, l.svcCtx.RedisClient)
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrGetSDKClient.String())
		return l.errorResponse(code.ErrGetSDKClient, err, nil), nil
	}

	// 异步调用上链默认返回交易 pending 状态
	pending := true

	// 不传则默认30秒
	txTimeout := in.TxTimeout
	if in.TxTimeout <= 0 {
		txTimeout = 30
	}
	txId, txData, err := sdkClient.SendTransaction(in.MethodType, in.ContractName, in.ContractMethod,
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
