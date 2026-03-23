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
	ErrUnknownContractType RespCode = iota + 600000
	ErrUnknownChainType
	ErrGetSDKClient
	ErrGetTxByTxId
	ErrSendTransaction
	ErrReadAbiJsonFile
	ErrChainNotExist
	ErrChainNotEnable
)

// 返回码对应具体的信息
var errMsg = map[RespCode]string{
	Success:                "success",
	ErrUnknownContractType: "unknown contract type",
	ErrUnknownChainType:    "unknown chain type",
	ErrGetSDKClient:        "failed to get sdk client",
	ErrGetTxByTxId:         "failed to get tx by tx id",
	ErrSendTransaction:     "failed to send transaction",
	ErrReadAbiJsonFile:     "failed to read abi json file",
	ErrChainNotExist:       "chain not exist",
	ErrChainNotEnable:      "chain not enable",
}

func (rc RespCode) String() string {
	return errMsg[rc]
}

const (
	ErrGetTxReceiptTimeoutMsg = "sync to get tx receipt timeout, maybe try it later"
)
