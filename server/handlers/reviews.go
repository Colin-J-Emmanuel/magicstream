package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Colin-J-Emmanuel/magicstream/server/llm"
	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

const (
	maxReviewLength = 2000
	classifyTimeout = 25 * time.Second
)

type ReviewHandler struct {
	movies     *mongo.Collection
	classifier *llm.Classifier
}

func NewReviewHandler(db *mongo.Database, classifier *llm.Classifier) *ReviewHandler {
	return &ReviewHandler{movies: db.Collection("movies"), classifier: classifier}
}

type reviewRequest struct {
	Review string `json:"review" binding:"required"`
}

// Update classifies a review and saves it together with its ranking.
// If classification fails, nothing is written.
func (h *ReviewHandler) Update(c *gin.Context) {
	// 1. Cheap checks first.
	imdbID := c.Param("imdb_id")
	if !imdbIDPattern.MatchString(imdbID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid imdb_id format"})
		return
	}
	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}
	review := strings.TrimSpace(req.Review)
	if review == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "review must not be empty"})
		return
	}
	if utf8.RuneCountInString(review) > maxReviewLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("review must be at most %d characters", maxReviewLength)})
		return
	}

	// 2. The movie must exist before we spend an LLM call on it.
	findCtx, findCancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer findCancel()
	var movie models.Movie
	err := h.movies.FindOne(findCtx, bson.M{"imdb_id": imdbID}).Decode(&movie)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusNotFound, gin.H{"error": "movie not found"})
		return
	}
	if err != nil {
		log.Printf("loading movie %s for review: %v", imdbID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save review"})
		return
	}

	// An identical, already-ranked review needs no new classification.
	if movie.AdminReview == review && movie.Ranking.RankingValue != models.NotRankedValue {
		c.JSON(http.StatusOK, movie)
		return
	}

	// 3. Classify before writing anything.
	llmCtx, llmCancel := context.WithTimeout(c.Request.Context(), classifyTimeout)
	defer llmCancel()
	ranking, err := h.classifier.Classify(llmCtx, review)
	if err != nil {
		log.Printf("classifying review for %s: %v", imdbID, err)
		status, msg := classifyFailure(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	// 4. One document, one write: review and ranking change together or not at all.
	writeCtx, writeCancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer writeCancel()
	var updated models.Movie
	err = h.movies.FindOneAndUpdate(writeCtx,
		bson.M{"imdb_id": imdbID},
		bson.M{"$set": bson.M{"admin_review": review, "ranking": ranking}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusNotFound, gin.H{"error": "movie not found"}) // deleted during classification
		return
	}
	if err != nil {
		log.Printf("saving review for %s: %v", imdbID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save review"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// classifyFailure maps an LLM error to an HTTP status and a client-safe message.
func classifyFailure(err error) (int, string) {
	var statusErr *llm.StatusError
	switch {
	case errors.Is(err, llm.ErrInvalidRanking):
		return http.StatusBadGateway, "the ranking service returned an unusable answer for this review"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "the ranking service took too long; please try again"
	case errors.As(err, &statusErr) && statusErr.StatusCode >= 400 && statusErr.StatusCode < 500 && statusErr.StatusCode != http.StatusTooManyRequests:
		return http.StatusInternalServerError, "the ranking service is misconfigured"
	default:
		return http.StatusServiceUnavailable, "the ranking service is unavailable; please try again later"
	}
}
