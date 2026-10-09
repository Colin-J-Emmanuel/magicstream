package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

// rankingValues is the fixed scale. 1 is best; values drive sort order.
var rankingValues = map[string]int{
	"Excellent": 1,
	"Good":      2,
	"Okay":      3,
	"Bad":       4,
	"Terrible":  5,
}

// ErrInvalidRanking means the model replied with something outside the scale.
var ErrInvalidRanking = errors.New("llm returned a ranking outside the allowed set")

const systemPrompt = "You classify movie reviews by sentiment. " +
	"Reply with exactly one word from this list and nothing else: Excellent, Good, Okay, Bad, Terrible."

// ParseRanking normalizes raw model output and accepts it only if it is exactly one allowed value.
func ParseRanking(raw string) (models.Ranking, error) {
	cleaned := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), ".!\"'*`"))
	for name, value := range rankingValues {
		if strings.EqualFold(cleaned, name) {
			return models.Ranking{RankingValue: value, RankingName: name}, nil
		}
	}
	return models.Ranking{}, fmt.Errorf("%w: %q", ErrInvalidRanking, truncate(raw, 100))
}

type Classifier struct {
	client *Client
}

func NewClassifier(client *Client) *Classifier {
	return &Classifier{client: client}
}

// Classify asks the model to rank a review and validates the answer.
func (cl *Classifier) Classify(ctx context.Context, review string) (models.Ranking, error) {
	raw, err := cl.client.Complete(ctx, []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Review: " + review},
	})
	if err != nil {
		return models.Ranking{}, err
	}
	return ParseRanking(raw)
}
