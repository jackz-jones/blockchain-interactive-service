package logic

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetTxByTxIdLogic 定义了查询交易详情逻辑执行对象
type GetTxByTxIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// NewGetTxByTxIdLogic 初始化查询交易详情逻辑执行对象
func NewGetTxByTxIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTxByTxIdLogic {
	return &GetTxByTxIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTxByTxId 查询交易详情
func (l *GetTxByTxIdLogic) GetTxByTxId(in *pb.GetTxByTxIdRequest) (*pb.TxResponse, error) {

	// 日志通用信息
	fields := map[string]interface{}{
		"requestId": in.RequestId,
		"txId":      in.TxId,
		"chainName": in.ChainName,
	}
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("receive GetTxByTxId request")

	// 检查 chainConf 是否存在
	chainConf, exist := l.svcCtx.Config.ChainConfs[in.ChainName]
	if !exist {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotExist.String())
		return l.errorResponse(code.ErrChainNotExist, nil), nil
	}

	// 如果链未启用，直接返回错误
	if !chainConf.Enable {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotEnable.String())
		return l.errorResponse(code.ErrChainNotEnable, nil), nil
	}

	// 获取sdk客户端
	sdkClient, err := sdk.GetSDKClient(&l.svcCtx.SDKClients, in.ChainName, l.Logger, chainConf,
		l.svcCtx.Config.Log, l.svcCtx.RedisClient)
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrGetSDKClient.String())
		return l.errorResponse(code.ErrGetSDKClient, err), nil
	}

	// 查询交易详情
	txData, pending, err := sdkClient.GetTxByTxId(in.TxId)
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrGetTxByTxId.String())
		return l.errorResponse(code.ErrGetTxByTxId, err), nil
	}

	// 返回成功信息
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("GetTxByTxId success return")
	return l.successResponse(&pb.TxData{
		ChainName: in.ChainName,
		Content:   txData,
		Pending:   pending,
	}), nil
}

// errorResponse returns the error response.
func (l *GetTxByTxIdLogic) errorResponse(code code.RespCode, err error) *pb.TxResponse {
	msg := code.String()
	if err != nil {
		msg = err.Error()
	}

	return &pb.TxResponse{
		Code: int32(code),
		Msg:  msg,
	}
}

// successResponse returns the success response.
func (l *GetTxByTxIdLogic) successResponse(data *pb.TxData) *pb.TxResponse {
	return &pb.TxResponse{
		Code: int32(code.Success),
		Msg:  code.Success.String(),
		Data: data,
	}
}
