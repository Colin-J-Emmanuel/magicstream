package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClassifySendsCorrectRequestAndParsesReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		if req.Model != "test-model" || len(req.Messages) != 2 || req.Temperature != 0 {
			t.Errorf("unexpected request: %+v", req)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Good"}}]}`)
	}))
	defer srv.Close()

	client, _ := NewClient(srv.URL, "test-key", "test-model")
	got, err := NewClassifier(client).Classify(context.Background(), "A solid, enjoyable film.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RankingName != "Good" || got.RankingValue != 2 {
		t.Fatalf("got %+v, want Good (2)", got)
	}
}

func TestClassifyRejectsOffScaleReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"I think it's pretty good overall!"}}]}`)
	}))
	defer srv.Close()

	client, _ := NewClient(srv.URL, "test-key", "test-model")
	_, err := NewClassifier(client).Classify(context.Background(), "whatever")
	if !errors.Is(err, ErrInvalidRanking) {
		t.Fatalf("want ErrInvalidRanking, got %v", err)
	}
}

func TestCompleteReturnsStatusErrorOnRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limit exceeded"}}`)
	}))
	defer srv.Close()

	client, _ := NewClient(srv.URL, "test-key", "test-model")
	_, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})

	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want StatusError 429, got %v", err)
	}
}

func TestCompleteRespectsContextDeadline(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // hang like a stuck provider until the test releases it
	}))
	defer srv.Close()
	defer close(release) // defers run last-in-first-out: this unblocks the handler, then Close returns at once

	client, _ := NewClient(srv.URL, "test-key", "test-model")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Complete(ctx, []Message{{Role: "user", Content: "hi"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("call took %v; the deadline was not enforced", elapsed)
	}
}
