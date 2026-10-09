package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Colin-J-Emmanuel/magicstream/server/database"
	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

func main() {
	_ = godotenv.Load()
	uri, dbName := os.Getenv("MONGODB_URI"), os.Getenv("DATABASE_NAME")
	if uri == "" || dbName == "" {
		log.Fatal("MONGODB_URI and DATABASE_NAME must be set")
	}

	raw, err := os.ReadFile("seed/movies.json")
	if err != nil {
		log.Fatalf("reading seed file: %v", err)
	}
	var movies []models.Movie
	if err := json.Unmarshal(raw, &movies); err != nil {
		log.Fatalf("parsing seed file: %v", err)
	}

	client, err := database.Connect(uri)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer client.Disconnect(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db := client.Database(dbName)
	if err := database.EnsureIndexes(ctx, db); err != nil {
		log.Fatalf("index setup failed: %v", err)
	}
	coll := db.Collection("movies")

	var inserted, updated int64
	for _, m := range movies {
		if m.ImdbID == "" {
			log.Fatalf("seed entry %q has no imdb_id", m.Title)
		}
		update := bson.M{
			// Catalog fields: owned by the seed file, updated on every run.
			"$set": bson.M{
				"title":       m.Title,
				"poster_path": m.PosterPath,
				"youtube_id":  m.YouTubeID,
				"genre":       m.Genre,
			},
			// Review fields: owned by the app, written only when the movie is first created.
			"$setOnInsert": bson.M{
				"admin_review": "",
				"ranking":      models.Ranking{RankingValue: models.NotRankedValue, RankingName: models.NotRankedName},
			},
		}
		res, err := coll.UpdateOne(ctx, bson.M{"imdb_id": m.ImdbID}, update,
			options.UpdateOne().SetUpsert(true))
		if err != nil {
			log.Fatalf("upserting %s: %v", m.ImdbID, err)
		}
		if res.UpsertedCount == 1 {
			inserted++
		} else if res.ModifiedCount == 1 {
			updated++
		}
	}
	unchanged := int64(len(movies)) - inserted - updated
	log.Printf("seed complete: %d inserted, %d updated, %d unchanged", inserted, updated, unchanged)
}
