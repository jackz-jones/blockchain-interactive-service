// Package sdk 提供与访问区块链的接口
package sdk

import (
	"context"

	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

type ChainSdkInterface interface {

	/**
	 * @Description: VerifyConnection 验证链连接是否真正可用
	 * 通过执行轻量级链查询（如获取链 ID / 最新区块高度 / 健康检查）来验证连接
	 *
	 * @param ctx 上下文（支持超时控制）
	 * @return error 连接不可用时返回错误
	 */

	VerifyConnection(ctx context.Context) error

	/**
	 * @Description: CallContract 调用合约

	 * @param methodType 合约调用类型：1（Invoke），2（Query）
	 * @param contractConfigName 合约配置名称
	 * @param method 合约方法名
	 * @param args 合约参数
	 * @param txTimeout 交易超时时间
	 * @param withSyncResult 是否同步等待交易执行结果

	 * @return string 交易哈希
	 * @return string 交易结果 json 字符串
	 * @return error 错误信息
	 */

	CallContract(methodType pb.MethodType, contractConfigName, method string, args []*pb.KeyValuePair,
		txTimeout int64, withSyncResult bool, gasLimit int64) (string, string, error)

	/**
	 * @Description: GetTxByTxId 根据交易id查询交易

	 * @param txId 交易哈希

	 * @return string 交易结果 json 字符串
	 * @return bool 交易是否打包块中，true 表示未打包（即 pending），false 表示已打包
	 * @return error 错误信息
	 */

	GetTxByTxId(txId string) (string, bool, error)

	/**
	 * @Description: Stop 释放 sdk 连接等资源

	* @return error 错误信息
	*/

	Stop() error

	/**
	 * @Description: SubscribeContractEvent 订阅合约事件
	 *
	 * @param contractConf 合约配置
	 * @param chainConfName 链配置名称
	 * @param contractConfName 合约配置名称
	 * @param chainType 链类型
	 * @param chainConfigID 链配置数据库主键 ID（DB 路径使用，配置文件路径传 0）
	 * @param contractConfigID 合约配置数据库主键 ID（DB 路径使用，配置文件路径传 0）
	 *
	 * @return error 错误信息
	 */

	SubscribeContractEvent(contractConf ContractConf, chainConfName, contractConfName,
		chainType string, chainConfigID, contractConfigID uint) error
}
