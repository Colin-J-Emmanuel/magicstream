package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseAccessRejectsExpiredToken(t *testing.T) {
	tm, err := NewTokenManager(strings.Repeat("a", 32), strings.Repeat("b", 32))
	if err != nil {
		t.Fatal(err)
	}

	// A negative TTL produces a token that expired a minute ago.
	expired, err := issue("user123", "USER", tm.accessSecret, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tm.ParseAccess(expired); !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}
