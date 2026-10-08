package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

var imdbIDPattern = regexp.MustCompile(`^tt\d{7,8}$`)

type MovieHandler struct {
	movies *mongo.Collection
}

func NewMovieHandler(db *mongo.Database) *MovieHandler {
	return &MovieHandler{movies: db.Collection("movies")}
}

// List returns all movies sorted by title.
func (h *MovieHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cursor, err := h.movies.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "title", Value: 1}}))
	if err != nil {
		log.Printf("listing movies: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch movies"})
		return
	}

	movies := []models.Movie{}
	if err := cursor.All(ctx, &movies); err != nil {
		log.Printf("decoding movies: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch movies"})
		return
	}
	c.JSON(http.StatusOK, movies)
}

// Get returns one movie by IMDb ID.
func (h *MovieHandler) Get(c *gin.Context) {
	imdbID := c.Param("imdb_id")
	if !imdbIDPattern.MatchString(imdbID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid imdb_id format"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var movie models.Movie
	err := h.movies.FindOne(ctx, bson.M{"imdb_id": imdbID}).Decode(&movie)
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusNotFound, gin.H{"error": "movie not found"})
		return
	}
	if err != nil {
		log.Printf("fetching movie %s: %v", imdbID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch movie"})
		return
	}
	c.JSON(http.StatusOK, movie)
}
