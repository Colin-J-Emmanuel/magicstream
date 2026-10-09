package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

type AdminHandler struct {
	users *mongo.Collection
}

func NewAdminHandler(db *mongo.Database) *AdminHandler {
	return &AdminHandler{users: db.Collection("users")}
}

// ListUsers returns every user, oldest first. Password hashes are excluded by the User type.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := h.users.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		log.Printf("listing users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users"})
		return
	}

	users := []models.User{}
	if err := cursor.All(ctx, &users); err != nil {
		log.Printf("decoding users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users"})
		return
	}
	c.JSON(http.StatusOK, users)
}
