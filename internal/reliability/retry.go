package reliability

import (
    "context"
    "math/rand"
    "time"
)

// RetryConfig controls retry behavior with exponential backoff and jitter.
type RetryConfig struct {
    MaxAttempts    int           // total attempts including the first
    InitialBackoff time.Duration // starting delay
    MaxBackoff     time.Duration // cap delay
    Multiplier     float64       // backoff growth factor per attempt (>1)
    Jitter         float64       // 0..1 fraction to randomize delay
}

// DefaultRetry returns a sane default retry config.
func DefaultRetry() RetryConfig {
    return RetryConfig{
        MaxAttempts:    5,
        InitialBackoff: 200 * time.Millisecond,
        MaxBackoff:     5 * time.Second,
        Multiplier:     2.0,
        Jitter:         0.2,
    }
}

// DoWithRetry runs fn with retries using cfg. If cb is not nil, calls are wrapped
// by the CircuitBreaker. Use isRetriable to decide which errors should be retried.
func DoWithRetry(
    ctx context.Context,
    cb *CircuitBreaker,
    cfg RetryConfig,
    isRetriable func(error) bool,
    fn func(context.Context) error,
) error {
    if cfg.MaxAttempts <= 0 {
        cfg = DefaultRetry()
    }
    attempt := 0
    backoff := cfg.InitialBackoff

    for {
        attempt++

        // Execute the function (optionally via circuit breaker)
        var err error
        if cb != nil {
            err = cb.Do(func() error { return fn(ctx) })
        } else {
            err = fn(ctx)
        }

        if err == nil {
            return nil
        }

        // If the breaker is open or error is not retriable, stop
        if err == ErrOpen || (isRetriable != nil && !isRetriable(err)) {
            return err
        }

        // Exhausted attempts
        if attempt >= cfg.MaxAttempts {
            return err
        }

        // Sleep with backoff and jitter, respect context cancellation
        delay := jitter(backoff, cfg.Jitter, cfg.MaxBackoff)
        t := time.NewTimer(delay)
        select {
        case <-ctx.Done():
            t.Stop()
            return ctx.Err()
        case <-t.C:
        }

        // Increase backoff for next attempt
        next := time.Duration(float64(backoff) * cfg.Multiplier)
        if next > cfg.MaxBackoff {
            next = cfg.MaxBackoff
        }
        backoff = next
    }
}

// jitter applies +/- jitter to d and caps to max.
func jitter(d time.Duration, frac float64, max time.Duration) time.Duration {
    if frac <= 0 {
        if d > max {
            return max
        }
        return d
    }
    // random in [-frac, +frac]
    f := (rand.Float64()*2 - 1) * frac
    jd := time.Duration(float64(d) * (1 + f))
    if jd < 0 {
        jd = 0
    }
    if jd > max {
        jd = max
    }
    return jd
}


