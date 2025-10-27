package reliability

import (
    "context"
    "errors"
    "sync"
    "time"
)

var ErrOpen = errors.New("circuit breaker open")

type CircuitBreaker struct {
    mu sync.Mutex

    failureThreshold int
    resetTimeout     time.Duration

    failures int
    state    string // "closed", "open", "half-open"
    openedAt time.Time
}

func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        failureThreshold: failureThreshold,
        resetTimeout:     resetTimeout,
        state:            "closed",
    }
}

func DefaultBreaker() *CircuitBreaker {
    return NewCircuitBreaker(5, 30*time.Second)
}

func (cb *CircuitBreaker) Do(fn func() error) error {
    cb.mu.Lock()
    switch cb.state {
    case "open":
        if time.Since(cb.openedAt) >= cb.resetTimeout {
            cb.state = "half-open"
        } else {
            cb.mu.Unlock()
            return ErrOpen
        }
    }
    cb.mu.Unlock()

    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()
    if err == nil {
        cb.failures = 0
        cb.state = "closed"
        return nil
    }

    cb.failures++
    if cb.failures >= cb.failureThreshold {
        cb.state = "open"
        cb.openedAt = time.Now()
    }
    return err
}

// DoWithRetry executes fn with retries using the provided RetryConfig, honoring
// the circuit breaker state. This is a convenience wrapper over DoWithRetry
// defined in retry.go that binds the breaker.
func (cb *CircuitBreaker) DoWithRetry(
    ctx context.Context,
    cfg RetryConfig,
    isRetriable func(error) bool,
    fn func(context.Context) error,
) error {
    return DoWithRetry(ctx, cb, cfg, isRetriable, fn)
}


