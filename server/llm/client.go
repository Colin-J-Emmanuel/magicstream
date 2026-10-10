package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy controls how transient failures are retried.
type RetryPolicy struct {
	MaxAttempts    int           // total attempts, including the first
	BaseDelay      time.Duration // ceiling for the first backoff; doubles each retry
	MaxDelay       time.Duration // cap on any single wait, including Retry-After
	AttemptTimeout time.Duration // per-attempt limit; 0 means only the caller's deadline applies
}

var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts:    3,
	BaseDelay:      500 * time.Millisecond,
	MaxDelay:       8 * time.Second,
	AttemptTimeout: 10 * time.Second,
}

// Client talks to any OpenAI-compatible chat completions endpoint.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	retry      RetryPolicy
}

func NewClient(baseURL, apiKey, model string) (*Client, error) {
	if baseURL == "" || apiKey == "" || model == "" {
		return nil, errors.New("LLM_BASE_URL, LLM_API_KEY and LLM_MODEL must all be set")
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		retry:      DefaultRetryPolicy,
	}, nil
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// StatusError is returned when the provider answers with a non-2xx status.
type StatusError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration // from the Retry-After header, if the provider sent one
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("llm provider returned %d: %s", e.StatusCode, e.Body)
}

// Complete sends the messages, retrying transient failures per the client's RetryPolicy.
func (c *Client) Complete(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages, Temperature: 0})
	if err != nil {
		return "", fmt.Errorf("encoding request: %w", err)
	}

	var lastErr error
	attempts := 0
	for attempts < c.retry.MaxAttempts {
		attempts++
		content, err := c.completeOnce(ctx, body)
		if err == nil {
			return content, nil
		}
		lastErr = err

		if !isRetryable(ctx, err) || attempts == c.retry.MaxAttempts {
			break
		}
		delay, ok := c.nextDelay(attempts, err)
		if !ok {
			break // provider asked us to wait longer than we're willing to
		}
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline && time.Until(deadline) < delay {
			break // waiting would outlast the caller's deadline: fail now instead of later
		}

		log.Printf("llm attempt %d/%d failed: %v; retrying in %v",
			attempts, c.retry.MaxAttempts, err, delay.Round(time.Millisecond))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return "", fmt.Errorf("llm call cancelled while waiting to retry: %w", ctx.Err())
		}
	}
	return "", fmt.Errorf("llm call failed after %d attempt(s): %w", attempts, lastErr)
}

// completeOnce makes a single HTTP attempt. The body is rebuilt each time
// because a request body is consumed when it's sent.
func (c *Client) completeOnce(ctx context.Context, body []byte) (string, error) {
	attemptCtx := ctx
	if c.retry.AttemptTimeout > 0 {
		var cancel context.CancelFunc
		attemptCtx, cancel = context.WithTimeout(ctx, c.retry.AttemptTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling llm provider: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", &StatusError{
			StatusCode: resp.StatusCode,
			Body:       truncate(string(respBody), 500),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("llm response contained no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// isRetryable reports whether sending the same request again could succeed.
func isRetryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false // the caller cancelled or their deadline passed: nobody is waiting
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true // only this attempt timed out (the caller is still waiting): try again
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= 500
	}
	var netErr net.Error
	return errors.As(err, &netErr) // connection refused/reset
}

// nextDelay returns how long to wait before the next attempt, and false if
// the provider asked for a longer wait than the policy allows.
func (c *Client) nextDelay(attempt int, err error) (time.Duration, bool) {
	var statusErr *StatusError
	if errors.As(err, &statusErr) && statusErr.RetryAfter > 0 {
		if statusErr.RetryAfter > c.retry.MaxDelay {
			return 0, false
		}
		return statusErr.RetryAfter, true
	}
	ceiling := min(c.retry.BaseDelay<<(attempt-1), c.retry.MaxDelay)
	return time.Duration(rand.Int64N(int64(ceiling) + 1)), true // full jitter: uniform in [0, ceiling]
}

// parseRetryAfter reads the delay-in-seconds form of Retry-After. The rarer
// HTTP-date form is ignored, falling back to normal backoff.
func parseRetryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
