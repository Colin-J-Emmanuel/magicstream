package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

// fastRetries keeps retry tests in milliseconds rather than seconds.
var fastRetries = RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}

// newTestClient points a client at a fake provider whose reply depends on the call number.
func newTestClient(t *testing.T, policy RetryPolicy, respond func(call int32, w http.ResponseWriter)) (*Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(calls.Add(1), w)
	}))
	t.Cleanup(srv.Close)

	client, _ := NewClient(srv.URL, "test-key", "test-model")
	client.retry = policy
	return client, &calls
}

func TestRetriesRateLimitThenSucceeds(t *testing.T) {
	client, calls := newTestClient(t, fastRetries, func(call int32, w http.ResponseWriter) {
		if call < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Good"}}]}`)
	})

	got, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil || got != "Good" {
		t.Fatalf("got %q, %v; want Good, nil", got, err)
	}
	if n := calls.Load(); n != 3 {
		t.Fatalf("provider saw %d calls, want 3 (two 429s, then success)", n)
	}
}

func TestDoesNotRetryUnauthorized(t *testing.T) {
	client, calls := newTestClient(t, fastRetries, func(call int32, w http.ResponseWriter) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want StatusError 401, got %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("provider saw %d calls, want 1: a bad key never improves", n)
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	client, calls := newTestClient(t, fastRetries, func(call int32, w http.ResponseWriter) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want StatusError 503 wrapped in the final error, got %v", err)
	}
	if !strings.Contains(err.Error(), "after 3 attempt(s)") {
		t.Fatalf("error should report the attempt count, got %v", err)
	}
	if n := calls.Load(); n != 3 {
		t.Fatalf("provider saw %d calls, want 3", n)
	}
}

func TestGivesUpWhenRetryAfterExceedsCap(t *testing.T) {
	client, calls := newTestClient(t, fastRetries, func(call int32, w http.ResponseWriter) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	start := time.Now()
	_, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("want an error")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("provider saw %d calls, want 1: retrying before 120s is a guaranteed failure", n)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v; should give up immediately, not wait", elapsed)
	}
}

func TestStopsWhenBackoffWouldOutlastDeadline(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Second}
	client, calls := newTestClient(t, policy, func(call int32, w http.ResponseWriter) {
		w.Header().Set("Retry-After", "2") // within the cap, but longer than the caller will wait
		w.WriteHeader(http.StatusTooManyRequests)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Complete(ctx, []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("want an error")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("provider saw %d calls, want 1", n)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("took %v; should fail at once rather than sleep past the deadline", elapsed)
	}
}

func TestRetriesNetworkErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close() // nothing is listening now: every attempt gets "connection refused"

	client, _ := NewClient(deadURL, "test-key", "test-model")
	client.retry = fastRetries

	_, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil || !strings.Contains(err.Error(), "after 3 attempt(s)") {
		t.Fatalf("want failure after 3 attempts, got %v", err)
	}
}

func TestOffScaleReplyIsNotRetried(t *testing.T) {
	client, calls := newTestClient(t, fastRetries, func(call int32, w http.ResponseWriter) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Amazing"}}]}`)
	})

	_, err := NewClassifier(client).Classify(context.Background(), "whatever")
	if !errors.Is(err, ErrInvalidRanking) {
		t.Fatalf("want ErrInvalidRanking, got %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("provider saw %d calls, want 1: at temperature 0 a retry repeats the same answer", n)
	}
}

func TestNextDelay(t *testing.T) {
	client := &Client{retry: RetryPolicy{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}}

	// Retry-After within the cap is used exactly.
	if d, ok := client.nextDelay(1, &StatusError{StatusCode: 429, RetryAfter: 500 * time.Millisecond}); !ok || d != 500*time.Millisecond {
		t.Fatalf("got %v, %v; want 500ms, true", d, ok)
	}
	// Jittered backoff stays within the doubling ceiling: 100ms, 200ms, 400ms...
	for attempt := 1; attempt <= 3; attempt++ {
		ceiling := 100 * time.Millisecond << (attempt - 1)
		for i := 0; i < 100; i++ {
			d, ok := client.nextDelay(attempt, errors.New("transient"))
			if !ok || d < 0 || d > ceiling {
				t.Fatalf("attempt %d: delay %v outside [0, %v]", attempt, d, ceiling)
			}
		}
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

func TestRetriesHungAttempt(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, AttemptTimeout: 50 * time.Millisecond}
	client, calls := newTestClient(t, policy, func(call int32, w http.ResponseWriter) {
		if call == 1 {
			time.Sleep(200 * time.Millisecond) // longer than AttemptTimeout: this attempt is abandoned
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Good"}}]}`)
	})

	got, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil || got != "Good" {
		t.Fatalf("got %q, %v; want Good after retrying the hung attempt", got, err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("provider saw %d calls, want 2", n)
	}
}
