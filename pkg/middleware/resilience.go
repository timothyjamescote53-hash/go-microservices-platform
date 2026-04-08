package middleware

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ─── Retry with Exponential Backoff ───────────────────────────────────────────

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Multiplier:  2.0,
	}
}

// Retry executes fn with exponential backoff. Retries on any non-nil error.
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt == cfg.MaxAttempts-1 {
			break
		}
		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// ─── Circuit Breaker ──────────────────────────────────────────────────────────

type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Failing, reject requests
	StateHalfOpen                     // Testing recovery
)

type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	successCount     int
	lastFailureTime  time.Time
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	logger           *zap.Logger
	name             string
}

var ErrCircuitOpen = errors.New("circuit breaker is open")

func NewCircuitBreaker(name string, failureThreshold, successThreshold int, timeout time.Duration, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		logger:           logger,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	state := cb.currentState()
	cb.mu.Unlock()

	if state == StateOpen {
		return ErrCircuitOpen
	}

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
	return err
}

func (cb *CircuitBreaker) currentState() CircuitState {
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.timeout {
		cb.state = StateHalfOpen
		cb.successCount = 0
		cb.logger.Info("Circuit breaker half-open", zap.String("name", cb.name))
	}
	return cb.state
}

func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
		cb.logger.Warn("Circuit breaker opened", zap.String("name", cb.name), zap.Int("failures", cb.failureCount))
	}
}

func (cb *CircuitBreaker) onSuccess() {
	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.logger.Info("Circuit breaker closed", zap.String("name", cb.name))
		}
	} else {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ─── HTTP Client with Retry + Circuit Breaker ─────────────────────────────────

type ResilientClient struct {
	client  *http.Client
	cb      *CircuitBreaker
	retry   RetryConfig
	logger  *zap.Logger
}

func NewResilientClient(name string, logger *zap.Logger) *ResilientClient {
	cb := NewCircuitBreaker(name, 5, 2, 30*time.Second, logger)
	return &ResilientClient{
		client: &http.Client{Timeout: 10 * time.Second},
		cb:     cb,
		retry:  DefaultRetryConfig(),
		logger: logger,
	}
}

func (c *ResilientClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := c.cb.Execute(func() error {
		return Retry(req.Context(), c.retry, func() error {
			var err error
			resp, err = c.client.Do(req)
			if err != nil {
				return err
			}
			if resp.StatusCode >= 500 {
				return errors.New("server error: " + resp.Status)
			}
			return nil
		})
	})
	return resp, err
}
// circuit breaker
// retry
// resilient client
// failure
// success
// timeout
