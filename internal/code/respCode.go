// Package code defines some response code
package code

// RespCode 服务端返回码
type RespCode int

// Success 服务端返回码，200 表示成功，其他表示失败
const (
	Success RespCode = 200000
)

// 600000-699999 表示 chain-interactive-service grpc 错误码
const (
	ErrUnknownChainType RespCode = iota + 600000
	ErrGetSDKClient
	ErrGetTxByTxId
	ErrSendTransaction
)

// 返回码对应具体的信息
var errMsg = map[RespCode]string{
	Success:             "success",
	ErrUnknownChainType: "unknown chain type",
	ErrGetSDKClient:     "failed to get sdk client",
	ErrGetTxByTxId:      "failed to get tx by tx id",
	ErrSendTransaction:  "failed to send transaction",
}

func (rc RespCode) String() string {
	return errMsg[rc]
}

const (
	ErrGetTxReceiptTimeoutMsg = "sync to get tx receipt timeout, maybe try it later"
)
