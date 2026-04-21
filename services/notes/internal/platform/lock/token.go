package lock

import (
	"crypto/rand"
	"encoding/hex"
)

// randomToken 生成 16 字节随机 token，用于锁持有者身份标识。
func randomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败极罕见；退化为空字符串时锁仍能被 TTL 释放
		return ""
	}
	return hex.EncodeToString(b)
}
