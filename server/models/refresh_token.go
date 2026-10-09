package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// RefreshToken is the server-side record of an issued refresh token.
// The token itself is never stored, only its SHA-256 hash.
type RefreshToken struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TokenHash string        `bson:"token_hash"`
	UserID    bson.ObjectID `bson:"user_id"`
	FamilyID  string        `bson:"family_id"`
	ExpiresAt time.Time     `bson:"expires_at"`
	UsedAt    *time.Time    `bson:"used_at"`
	CreatedAt time.Time     `bson:"created_at"`
}
