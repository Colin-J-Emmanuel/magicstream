package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Connect creates a Mongo client and proves the server is reachable.
func Connect(uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("creating mongo client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("pinging mongo: %w", err)
	}
	return client, nil
}

// EnsureIndexes creates the indexes the app relies on. Safe to run on every startup.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("movies").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "imdb_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("creating unique index on movies.imdb_id: %w", err)
	}
	return nil
}
