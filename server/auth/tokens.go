package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
)

// Claims is the JWT payload. Everything here is readable by anyone holding the token.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
}

func NewTokenManager(accessSecret, refreshSecret string) (*TokenManager, error) {
	if len(accessSecret) < 32 || len(refreshSecret) < 32 {
		return nil, errors.New("token secrets must each be at least 32 characters")
	}
	if accessSecret == refreshSecret {
		return nil, errors.New("access and refresh secrets must be different")
	}
	return &TokenManager{accessSecret: []byte(accessSecret), refreshSecret: []byte(refreshSecret)}, nil
}

func (tm *TokenManager) IssueAccess(userID, role string) (string, error) {
	return issue(userID, role, tm.accessSecret, AccessTTL)
}

func (tm *TokenManager) IssueRefresh(userID, role string) (string, error) {
	return issue(userID, role, tm.refreshSecret, RefreshTTL)
}

func issue(userID, role string, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}
