package auth

import (
	"strings"
	"testing"
)

// 由 Python hashlib.pbkdf2_hmac 独立生成，验证与 v1 格式及标准 PBKDF2 互通。
const v1Fixture = "pbkdf2-sha256$120000$MDEyMzQ1Njc4OWFiY2RlZg$1FcR4VSgaaoV/en0r0RXZLFpUnLvdIsWsficFQGt/Ww"

func TestCheckPasswordCompatibleWithV1Format(t *testing.T) {
	if !CheckPassword(v1Fixture, "correct horse") {
		t.Fatal("应能验证 v1 格式哈希")
	}
	if CheckPassword(v1Fixture, "wrong") {
		t.Fatal("错误密码不应通过")
	}
}

func TestHashRoundTripAndRandomSalt(t *testing.T) {
	h1, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := HashPassword("s3cret-pass")
	if h1 == h2 {
		t.Fatal("两次哈希应使用不同的盐")
	}
	if !strings.HasPrefix(h1, "pbkdf2-sha256$120000$") || !CheckPassword(h1, "s3cret-pass") || CheckPassword(h1, "S3cret-pass") {
		t.Fatalf("哈希往返失败: %s", h1)
	}
}

func TestCheckPasswordRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "plain", "pbkdf2-sha256$x$y$z", "pbkdf2-sha256$0$AAAA$AAAA", "md5$1$a$b", "pbkdf2-sha256$1$!!$AAAA"} {
		if CheckPassword(bad, "x") {
			t.Errorf("损坏的哈希 %q 不应通过", bad)
		}
	}
}

func TestTokens(t *testing.T) {
	a, _ := NewToken()
	b, _ := NewToken()
	if a == b || len(a) < 40 {
		t.Fatalf("令牌异常: %s %s", a, b)
	}
	if HashToken(a) == HashToken(b) || len(HashToken(a)) != 64 {
		t.Fatal("令牌哈希异常")
	}
	if CheckDummy("x") {
		t.Fatal("CheckDummy 应恒为 false")
	}
}
