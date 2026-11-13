// Package auth 负责密码哈希与会话令牌。
//
// 哈希格式 pbkdf2-sha256$<迭代>$<salt b64>$<hash b64>，与 v1 完全兼容，
// 迁移过来的账号无需重置密码。
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	iterations = 120000
	saltBytes  = 16
	keyBytes   = 32
	tokenBytes = 32
)

// HashPassword 生成带随机盐的密码哈希。
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// CheckPassword 以恒定时间比较密码与哈希。格式损坏一律视为不匹配。
func CheckPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 || iter > 1_000_000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash 用于用户不存在时也做一次同样开销的校验，避免通过响应时间枚举用户名。
var dummyHash, _ = HashPassword("not-a-real-password")

// CheckDummy 执行一次与真实校验等价开销的计算，结果恒为 false。
func CheckDummy(password string) bool { return CheckPassword(dummyHash, password) && false }

// NewToken 生成会话令牌（URL 安全 base64）。
func NewToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken 返回令牌的 sha256 十六进制，数据库只存这个，泄库不泄令牌。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
