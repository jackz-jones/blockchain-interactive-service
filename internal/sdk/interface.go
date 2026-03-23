// Package sdk 提供与访问区块链的接口
package sdk

import (
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

type ChainSdkInterface interface {

	/**
	 * @Description: SendTransaction 调用 eth_sendTransaction 方法，交易执行状态会上链，一般用于写数据类型调用

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

	SendTransaction(methodType pb.MethodType, contractConfigName, method string, args []*pb.KeyValuePair,
		txTimeout int64, withSyncResult bool) (string, string, error)

	/**
	 * @Description: GetTxByTxId 根据交易id查询交易

	 * @param txId 交易哈希

	 * @return string 交易结果 json 字符串
	 * @return bool 交易是否打包块中
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
	 * @param contractType 合约类型
	 *
	 * @return error 错误信息
	 */

	SubscribeContractEvent(contractConf config.ContractConf, chainConfName, contractConfName, chainType,
		contractType string) error
}
