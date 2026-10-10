package handlers

import (
	"context"
	"errors"
	"fmt"
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

// refreshCookiePath scopes the refresh cookie to the auth routes. It must match  // CHANGED: comment added
// where those routes are mounted, or the browser silently stops sending it.
const refreshCookiePath = "/api/auth"

type AuthHandler struct {
	users         *mongo.Collection
	refreshTokens *mongo.Collection
	tokens        *auth.TokenManager
	secureCookies bool
}

func NewAuthHandler(db *mongo.Database, tokens *auth.TokenManager, secureCookies bool) *AuthHandler {
	return &AuthHandler{
		users:         db.Collection("users"),
		refreshTokens: db.Collection("refresh_tokens"),
		tokens:        tokens,
		secureCookies: secureCookies,
	}
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

	familyID, err := auth.NewID()
	if err != nil {
		log.Printf("generating session family id: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	if err := h.issueSession(ctx, c, user, familyID); err != nil {
		log.Printf("starting session for %s: %v", user.ID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, access, refresh string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "access_token", Value: access, Path: "/",
		MaxAge:   int(auth.AccessTTL.Seconds()),
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name: "refresh_token", Value: refresh, Path: refreshCookiePath,
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

// issueSession issues a token pair, records the refresh token's hash, and sets cookies.
func (h *AuthHandler) issueSession(ctx context.Context, c *gin.Context, user models.User, familyID string) error {
	access, err := h.tokens.IssueAccess(user.ID.Hex(), string(user.Role))
	if err != nil {
		return fmt.Errorf("issuing access token: %w", err)
	}
	refresh, err := h.tokens.IssueRefresh(user.ID.Hex(), string(user.Role))
	if err != nil {
		return fmt.Errorf("issuing refresh token: %w", err)
	}

	now := time.Now().UTC()
	_, err = h.refreshTokens.InsertOne(ctx, models.RefreshToken{
		TokenHash: auth.HashToken(refresh),
		UserID:    user.ID,
		FamilyID:  familyID,
		ExpiresAt: now.Add(auth.RefreshTTL),
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("storing refresh token: %w", err)
	}

	h.setAuthCookies(c, access, refresh)
	return nil
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	raw, err := c.Cookie("refresh_token")
	if err != nil || raw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	claims, err := h.tokens.ParseRefresh(raw)
	if err != nil {
		log.Printf("rejected refresh token: %v", err)
		h.clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	hash := auth.HashToken(raw)

	// Atomically claim the token: of any concurrent requests, only one can mark it used.
	var stored models.RefreshToken
	err = h.refreshTokens.FindOneAndUpdate(ctx,
		bson.M{"token_hash": hash, "used_at": nil},
		bson.M{"$set": bson.M{"used_at": time.Now().UTC()}},
	).Decode(&stored)

	if errors.Is(err, mongo.ErrNoDocuments) {
		// Not claimable: either unknown/revoked, or already used. Already used means reuse, a likely theft.
		var prior models.RefreshToken
		if findErr := h.refreshTokens.FindOne(ctx, bson.M{"token_hash": hash}).Decode(&prior); findErr == nil {
			log.Printf("refresh token reuse detected for user %s; revoking family %s", prior.UserID.Hex(), prior.FamilyID)
			if _, delErr := h.refreshTokens.DeleteMany(ctx, bson.M{"family_id": prior.FamilyID}); delErr != nil {
				log.Printf("revoking family %s: %v", prior.FamilyID, delErr)
			}
		}
		h.clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if err != nil {
		log.Printf("claiming refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refresh failed"})
		return
	}
	if claims.Subject != stored.UserID.Hex() {
		log.Printf("refresh token subject mismatch: claims %s, stored %s", claims.Subject, stored.UserID.Hex())
		h.clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	// Reload the user so a changed role or deleted account takes effect on refresh.
	var user models.User
	err = h.users.FindOne(ctx, bson.M{"_id": stored.UserID}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		h.clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if err != nil {
		log.Printf("loading user for refresh: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refresh failed"})
		return
	}

	if err := h.issueSession(ctx, c, user, stored.FamilyID); err != nil {
		log.Printf("rotating session for %s: %v", user.ID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refresh failed"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// Logout revokes the session's whole token family and clears cookies. It always succeeds.
func (h *AuthHandler) Logout(c *gin.Context) {
	if raw, err := c.Cookie("refresh_token"); err == nil && raw != "" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		var stored models.RefreshToken
		err := h.refreshTokens.FindOne(ctx, bson.M{"token_hash": auth.HashToken(raw)}).Decode(&stored)
		if err == nil {
			if _, delErr := h.refreshTokens.DeleteMany(ctx, bson.M{"family_id": stored.FamilyID}); delErr != nil {
				log.Printf("revoking family on logout: %v", delErr)
			}
		} else if !errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("looking up refresh token on logout: %v", err)
		}
	}
	h.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}

// clearAuthCookies expires both cookies. Path must match how they were set, or the browser keeps them.
func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	for _, ck := range []struct{ name, path string }{{"access_token", "/"}, {"refresh_token", refreshCookiePath}} {
		http.SetCookie(c.Writer, &http.Cookie{
			Name: ck.name, Value: "", Path: ck.path, MaxAge: -1,
			HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
		})
	}
}
