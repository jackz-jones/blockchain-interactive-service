// Package util defines some common function
package util

import (
	"fmt"
	"os"

	"github.com/zeromicro/go-zero/core/logx"
)

// ConvertToLogFields 将 map 转成 logField
func ConvertToLogFields(fields map[string]interface{}) []logx.LogField {
	res := make([]logx.LogField, 0)
	for k, v := range fields {

		// logx.Field 本身就是 kv
		res = append(res, logx.Field(k, v))
	}

	return res
}

// ReadAbiJsonFile 读取abi json文件
func ReadAbiJsonFile(abiJsonFile string) (string, error) {

	// 读取abi文件
	abiJson, err := os.ReadFile(abiJsonFile)
	if err != nil {
		return "", fmt.Errorf("failed to ReadFile: %v", err)
	}

	return string(abiJson), nil
}
