package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/Colin-J-Emmanuel/magicstream/server/llm"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal(`usage: go run ./cmd/classify "review text"`)
	}
	_ = godotenv.Load()

	client, err := llm.NewClient(os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_API_KEY"), os.Getenv("LLM_MODEL"))
	if err != nil {
		log.Fatalf("llm setup failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ranking, err := llm.NewClassifier(client).Classify(ctx, strings.Join(os.Args[1:], " "))
	if err != nil {
		log.Fatalf("classification failed: %v", err)
	}
	fmt.Printf("%s (%d)\n", ranking.RankingName, ranking.RankingValue)
}
