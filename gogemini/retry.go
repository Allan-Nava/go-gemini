package gogemini

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"time"
)

// RetryPolicy controls how the client retries transient failures: an APIError
// whose Retryable method reports true (408, 429, 5xx), a network error, or an
// attempt that ran out of time (see WithTimeout). Nothing is retried once the
// caller's context is done, and ErrRedirectOtherHost never is.
//
// The wait before retry n is InitialDelay·2ⁿ⁻¹, capped at MaxDelay, with random
// jitter between half and all of it. When Google sends a RetryDelay the client
// waits at least that long, and gives up instead if it is longer than MaxDelay.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, the first one included.
	// 1 disables retries.
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

// DefaultRetryPolicy returns the policy a client uses without WithRetry:
// 4 attempts, waiting from 1 s up to 30 s.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 4, InitialDelay: time.Second, MaxDelay: 30 * time.Second}
}

func (p RetryPolicy) validate() error {
	switch {
	case p.MaxAttempts < 1:
		return fmt.Errorf("%w: MaxAttempts must be at least 1, got %d", ErrInvalidRetryPolicy, p.MaxAttempts)
	case p.MaxAttempts == 1:
		return nil
	case p.InitialDelay <= 0:
		return fmt.Errorf("%w: InitialDelay must be positive", ErrInvalidRetryPolicy)
	case p.MaxDelay < p.InitialDelay:
		return fmt.Errorf("%w: MaxDelay must be at least InitialDelay", ErrInvalidRetryPolicy)
	}
	return nil
}

// wait returns how long to sleep before retry number n (1-based), or false to give up.
func (p RetryPolicy) wait(n int, serverDelay time.Duration) (time.Duration, bool) {
	if serverDelay > p.MaxDelay {
		return 0, false
	}
	d := p.InitialDelay
	for i := 1; i < n && d < p.MaxDelay; i++ {
		d *= 2
	}
	d = min(d, p.MaxDelay)
	d = d/2 + rand.N(d/2+1) // jitter in [d/2, d]
	return max(d, serverDelay), true
}

// retryable reports whether err is worth another attempt. Once the caller's ctx is
// done nothing is; an attempt that hit the client's own timeout is.
func retryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, ErrRedirectOtherHost) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable()
	}
	var urlErr *url.Error
	return errors.Is(err, context.DeadlineExceeded) || errors.As(err, &urlErr) // no answer
}

func serverDelay(err error) time.Duration {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.RetryDelay
	}
	return 0
}

// sleep waits for d or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
