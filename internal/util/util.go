// Package util defines some common function
package util

import (
	"fmt"
	"os"
	"strings"

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

// ReadAbiJsonFile 读取abi json内容
// 支持两种模式：
// 1. 如果 abiJsonOrFile 是 JSON 字符串（以 [ 或 { 开头），直接返回
// 2. 否则当作文件路径读取
func ReadAbiJsonFile(abiJsonOrFile string) (string, error) {
	trimmed := strings.TrimSpace(abiJsonOrFile)
	// 如果内容以 [ 或 { 开头，说明是直接存储的 ABI JSON 字符串
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		return trimmed, nil
	}

	// 否则当作文件路径读取
	abiJson, err := os.ReadFile(abiJsonOrFile)
	if err != nil {
		return "", fmt.Errorf("failed to ReadFile: %v", err)
	}

	return string(abiJson), nil
}
