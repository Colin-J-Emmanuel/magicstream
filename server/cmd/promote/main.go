package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/Colin-J-Emmanuel/magicstream/server/database"
	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

func main() {
	email := flag.String("email", "", "email of the user to update")
	role := flag.String("role", "ADMIN", "role to assign: ADMIN or USER")
	flag.Parse()

	if *email == "" {
		log.Fatal("usage: go run ./cmd/promote -email you@example.com [-role ADMIN|USER]")
	}
	newRole := models.Role(strings.ToUpper(*role))
	if newRole != models.RoleAdmin && newRole != models.RoleUser {
		log.Fatalf("invalid role %q: must be ADMIN or USER", *role)
	}

	_ = godotenv.Load()
	uri, dbName := os.Getenv("MONGODB_URI"), os.Getenv("DATABASE_NAME")
	if uri == "" || dbName == "" {
		log.Fatal("MONGODB_URI and DATABASE_NAME must be set")
	}

	client, err := database.Connect(uri)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer client.Disconnect(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	normalized := strings.ToLower(strings.TrimSpace(*email))
	res, err := client.Database(dbName).Collection("users").UpdateOne(ctx,
		bson.M{"email": normalized},
		bson.M{"$set": bson.M{"role": newRole}},
	)
	if err != nil {
		log.Fatalf("updating role: %v", err)
	}
	if res.MatchedCount == 0 {
		log.Fatalf("no user with email %s", normalized)
	}
	if res.ModifiedCount == 0 {
		log.Printf("%s already has role %s; nothing changed", normalized, newRole)
		return
	}
	log.Printf("%s is now %s", normalized, newRole)
	log.Println("takes effect at the user's next login or token refresh (within 15 minutes)")
}
