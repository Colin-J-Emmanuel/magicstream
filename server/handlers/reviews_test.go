package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Colin-J-Emmanuel/magicstream/server/llm"
)

func TestClassifyFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"off-scale answer", fmt.Errorf("parsing: %w", llm.ErrInvalidRanking), http.StatusBadGateway},
		{"deadline", fmt.Errorf("llm call failed after 1 attempt(s): %w", context.DeadlineExceeded), http.StatusGatewayTimeout},
		{"bad api key", fmt.Errorf("llm call failed after 1 attempt(s): %w", &llm.StatusError{StatusCode: 401}), http.StatusInternalServerError},
		{"rate limited past retries", fmt.Errorf("llm call failed after 3 attempt(s): %w", &llm.StatusError{StatusCode: 429}), http.StatusServiceUnavailable},
		{"unreachable", errors.New("connection refused"), http.StatusServiceUnavailable},
		{"rejected by provider (400)", fmt.Errorf("llm call failed after 1 attempt(s): %w", &llm.StatusError{StatusCode: 400}), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := classifyFailure(tt.err); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
