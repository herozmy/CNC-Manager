package domain

import (
	"context"
	"testing"
	"time"
)

func TestNowIsUTCRFC3339(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, Now())
	if err != nil || ts.Location() != time.UTC {
		t.Fatalf("Now() = %q 不是 UTC RFC3339: %v", Now(), err)
	}
}

func TestUserContextRoundTrip(t *testing.T) {
	if _, ok := UserFromContext(context.Background()); ok {
		t.Fatal("空上下文不应有用户")
	}
	u := User{ID: 3, Username: "a"}
	got, ok := UserFromContext(WithUser(context.Background(), u))
	if !ok || got != u {
		t.Fatalf("取回用户不一致: %+v", got)
	}
}
