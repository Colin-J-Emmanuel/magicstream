package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"

	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

// bcryptCost 12 takes a few hundred ms per hash: negligible for one login,
// prohibitive for brute-forcing a stolen database.
const bcryptCost = 12

type AuthHandler struct {
	users *mongo.Collection
}

func NewAuthHandler(db *mongo.Database) *AuthHandler {
	return &AuthHandler{users: db.Collection("users")}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at most 72 bytes"})
		return
	}
	if err != nil {
		log.Printf("hashing password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	user := models.User{
		FirstName:      strings.TrimSpace(req.FirstName),
		LastName:       strings.TrimSpace(req.LastName),
		Email:          strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash:   string(hash),
		Role:           models.RoleUser, // always USER: set by the server, never the client
		FavoriteGenres: req.FavoriteGenres,
		CreatedAt:      time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	res, err := h.users.InsertOne(ctx, user)
	if mongo.IsDuplicateKeyError(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "an account with this email already exists"})
		return
	}
	if err != nil {
		log.Printf("inserting user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	user.ID = res.InsertedID.(bson.ObjectID)
	c.JSON(http.StatusCreated, user)
}
