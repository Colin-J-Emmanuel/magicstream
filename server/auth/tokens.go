package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	id, err := NewID()
	if err != nil {
		return "", fmt.Errorf("generating token id: %w", err)
	}
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        id,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func (tm *TokenManager) ParseAccess(token string) (*Claims, error) {
	return parse(token, tm.accessSecret)
}

func (tm *TokenManager) ParseRefresh(token string) (*Claims, error) {
	return parse(token, tm.refreshSecret)
}

func parse(tokenString string, secret []byte) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // pin the algorithm
		jwt.WithExpirationRequired(),                                 // no exp claim = invalid
	)
	if err != nil {
		return nil, err
	}
	if claims.Subject == "" {
		return nil, errors.New("token has no subject")
	}
	return claims, nil
}

// NewID returns a random 128-bit identifier, hex-encoded.
func NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of a token. Refresh tokens are stored only as hashes.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
