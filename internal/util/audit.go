package util

import (
	"encoding/json"
)

// CompareBeforeAfter 逐字段比较变更前后的 map 数据是否一致
// 解决 json.Marshal(map) 字段顺序不确定、无法直接比较字符串的问题
func CompareBeforeAfter(before, after map[string]interface{}) bool {
	if len(before) != len(after) {
		return false
	}
	for key, afterVal := range after {
		beforeVal, exists := before[key]
		if !exists || beforeVal != afterVal {
			return false
		}
	}
	return true
}

// BuildAuditDetailJSON 构建变更前后对比的审计详情 JSON
// 使用 json.MarshalIndent 生成格式化的 JSON，便于审计日志阅读
// 注：Go 标准库 json.Marshal 对 map 的 key 按字母序排列，输出顺序稳定
func BuildAuditDetailJSON(before, after map[string]interface{}) string {
	auditDetail := map[string]interface{}{
		"before": before,
		"after":  after,
	}

	bytes, err := json.MarshalIndent(auditDetail, "", "  ")
	if err != nil {
		// 降级：无缩进序列化
		bytes, _ = json.Marshal(auditDetail)
	}
	return string(bytes)
}
