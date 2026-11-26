package middleware

import (
	"crypto/sha256"
)

// hashRawKey 计算原始 API Key 的 SHA-256 摘要，用于常量时间比较
func hashRawKey(rawKey string) [32]byte {
	return sha256.Sum256([]byte(rawKey))
}
