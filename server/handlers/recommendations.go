package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Colin-J-Emmanuel/magicstream/server/middleware"
	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

const (
	defaultRecommendationLimit = 10
	maxRecommendationLimit     = 50
	// A recommendation is an endorsement: only Okay (3) or better. This also
	// excludes unranked movies, whose sentinel value is 999.
	worstRecommendedRank = 3
)

type RecommendationHandler struct {
	users  *mongo.Collection
	movies *mongo.Collection
}

func NewRecommendationHandler(db *mongo.Database) *RecommendationHandler {
	return &RecommendationHandler{users: db.Collection("users"), movies: db.Collection("movies")}
}

// List returns movies in the caller's favorite genres, best-ranked first.
func (h *RecommendationHandler) List(c *gin.Context) {
	limit := defaultRecommendationLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxRecommendationLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("limit must be an integer from 1 to %d", maxRecommendationLimit)})
			return
		}
		limit = n
	}

	userID, err := bson.ObjectIDFromHex(middleware.UserID(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Favorite genres come from the database, not the token: they can change after login.
	var user models.User
	err = h.users.FindOne(ctx, bson.M{"_id": userID},
		options.FindOne().SetProjection(bson.M{"favorite_genres": 1}),
	).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if err != nil {
		log.Printf("loading favorite genres for %s: %v", userID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch recommendations"})
		return
	}

	genreIDs := make([]int, 0, len(user.FavoriteGenres))
	for _, g := range user.FavoriteGenres {
		genreIDs = append(genreIDs, g.GenreID)
	}
	if len(genreIDs) == 0 {
		c.JSON(http.StatusOK, []models.Movie{})
		return
	}

	filter := bson.M{
		"genre.genre_id":        bson.M{"$in": genreIDs},
		"ranking.ranking_value": bson.M{"$lte": worstRecommendedRank},
	}
	opts := options.Find().
		SetSort(bson.D{
			{Key: "ranking.ranking_value", Value: 1}, // best first
			{Key: "title", Value: 1},                 // deterministic tie-break
		}).
		SetLimit(int64(limit))

	cursor, err := h.movies.Find(ctx, filter, opts)
	if err != nil {
		log.Printf("querying recommendations for %s: %v", userID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch recommendations"})
		return
	}
	movies := []models.Movie{}
	if err := cursor.All(ctx, &movies); err != nil {
		log.Printf("decoding recommendations for %s: %v", userID.Hex(), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch recommendations"})
		return
	}
	c.JSON(http.StatusOK, movies)
}
