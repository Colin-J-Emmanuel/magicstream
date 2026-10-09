package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Role string

const (
	RoleUser  Role = "USER"
	RoleAdmin Role = "ADMIN"
)

// User is the stored document.
type User struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"_id"`
	FirstName      string        `bson:"first_name" json:"first_name"`
	LastName       string        `bson:"last_name" json:"last_name"`
	Email          string        `bson:"email" json:"email"`
	PasswordHash   string        `bson:"password_hash" json:"-"`
	Role           Role          `bson:"role" json:"role"`
	FavoriteGenres []Genre       `bson:"favorite_genres" json:"favorite_genres"`
	CreatedAt      time.Time     `bson:"created_at" json:"created_at"`
}

// RegisterRequest is what a client may send. It has no Role field,
// so a client cannot assign itself a role.
type RegisterRequest struct {
	FirstName      string  `json:"first_name" binding:"required,max=50"`
	LastName       string  `json:"last_name" binding:"required,max=50"`
	Email          string  `json:"email" binding:"required,email,max=254"`
	Password       string  `json:"password" binding:"required,min=8,max=72"`
	FavoriteGenres []Genre `json:"favorite_genres" binding:"required,min=1"`
}
