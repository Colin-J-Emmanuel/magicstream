package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/Colin-J-Emmanuel/magicstream/server/auth"
	"github.com/Colin-J-Emmanuel/magicstream/server/database"
	"github.com/Colin-J-Emmanuel/magicstream/server/handlers"
	"github.com/Colin-J-Emmanuel/magicstream/server/llm"
	"github.com/Colin-J-Emmanuel/magicstream/server/middleware"
	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found; using environment variables")
	}

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("MONGODB_URI is not set")
	}

	client, err := database.Connect(uri)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("error disconnecting from mongo: %v", err)
		}
	}()

	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		log.Fatal("DATABASE_NAME is not set")
	}
	db := client.Database(dbName)

	idxCtx, idxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer idxCancel()
	if err := database.EnsureIndexes(idxCtx, db); err != nil {
		log.Fatalf("index setup failed: %v", err)
	}

	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Fatalf("failed to set trusted proxies: %v", err)
	}

	router.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := client.Ping(ctx, nil); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "database": "unreachable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "ok"})
	})

	movieHandler := handlers.NewMovieHandler(db)
	router.GET("/movies", movieHandler.List)
	router.GET("/movies/:imdb_id", movieHandler.Get)

	tokens, err := auth.NewTokenManager(os.Getenv("ACCESS_TOKEN_SECRET"), os.Getenv("REFRESH_TOKEN_SECRET"))
	if err != nil {
		log.Fatalf("token setup failed: %v", err)
	}

	llmClient, err := llm.NewClient(os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_API_KEY"), os.Getenv("LLM_MODEL"))
	if err != nil {
		log.Fatalf("llm setup failed: %v", err)
	}

	secureCookies := os.Getenv("COOKIE_SECURE") == "true"

	authHandler := handlers.NewAuthHandler(db, tokens, secureCookies)
	authRoutes := router.Group("/auth")
	authRoutes.POST("/register", authHandler.Register)
	authRoutes.POST("/login", authHandler.Login)
	authRoutes.POST("/refresh", authHandler.Refresh)
	authRoutes.POST("/logout", authHandler.Logout)

	protected := router.Group("/", middleware.RequireAuth(tokens))
	protected.GET("/me", authHandler.Me)

	adminHandler := handlers.NewAdminHandler(db)
	admin := protected.Group("/admin", middleware.RequireRole(models.RoleAdmin))
	admin.GET("/users", adminHandler.ListUsers)

	reviewHandler := handlers.NewReviewHandler(db, llm.NewClassifier(llmClient))
	admin.PATCH("/movies/:imdb_id/review", reviewHandler.Update)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
