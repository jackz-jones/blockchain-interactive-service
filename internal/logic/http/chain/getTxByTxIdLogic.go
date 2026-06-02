package chain

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTxByTxIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTxByTxIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTxByTxIdLogic {
	return &GetTxByTxIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTxByTxIdLogic) GetTxByTxId(req *types.GetTxByTxIdRequest) (resp *types.CommonResponse, err error) {
	if req.TxId == "" || req.ChainName == "" {
		return &types.CommonResponse{Code: 400, Message: "txId path param and chain_name query param are required"}, nil
	}

	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	client, err := l.svcCtx.TenantSDKManager.GetTenantSDKClient(l.ctx, tenantID, req.ChainName)
	if err != nil {
		return &types.CommonResponse{Code: 400, Message: "get sdk client: " + err.Error()}, nil
	}

	result, confirmed, err := client.GetTxByTxId(req.TxId)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get tx: " + err.Error()}, nil
	}

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"result":    result,
			"confirmed": confirmed,
		},
	}, nil
}
