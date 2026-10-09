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

	"github.com/Colin-J-Emmanuel/magicstream/server/auth"
	"github.com/Colin-J-Emmanuel/magicstream/server/middleware"
	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

// bcryptCost 12 takes a few hundred ms per hash: negligible for one login,
// prohibitive for brute-forcing a stolen database.
const bcryptCost = 12

type AuthHandler struct {
	users         *mongo.Collection
	tokens        *auth.TokenManager
	secureCookies bool
}

func NewAuthHandler(db *mongo.Database, tokens *auth.TokenManager, secureCookies bool) *AuthHandler {
	return &AuthHandler{users: db.Collection("users"), tokens: tokens, secureCookies: secureCookies}
}

// dummyHash is compared against when an email doesn't exist, so a missing
// account takes as long as a wrong password and timing reveals nothing.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcryptCost)

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

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var user models.User
	email := strings.ToLower(strings.TrimSpace(req.Email))
	err := h.users.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password)) // equalize timing
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		log.Printf("finding user for login: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		log.Printf("comparing password hash for %s: %v", user.ID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	access, err := h.tokens.IssueAccess(user.ID.Hex(), string(user.Role))
	if err != nil {
		log.Printf("issuing access token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	refresh, err := h.tokens.IssueRefresh(user.ID.Hex(), string(user.Role))
	if err != nil {
		log.Printf("issuing refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	h.setAuthCookies(c, access, refresh)
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, access, refresh string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "access_token", Value: access, Path: "/",
		MaxAge:   int(auth.AccessTTL.Seconds()),
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "refresh_token", Value: refresh, Path: "/auth",
		MaxAge:   int(auth.RefreshTTL.Seconds()),
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var user models.User
	err = h.users.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if err != nil {
		log.Printf("fetching current user %s: %v", id.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}
	c.JSON(http.StatusOK, user)
}
