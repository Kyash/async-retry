package asyncretry

import (
	"context"
	"fmt"
	"sync"

	"github.com/avast/retry-go/v4"
)

type RetryableFunc func(ctx context.Context) error
type FinishFunc func(error)

type AsyncRetry interface {
	// Do calls f in a new goroutine, and retry if necessary. When finished, `finish` is called regardless of success or failure exactly once.
	// Non-nil error is always `ErrInShutdown` that will be returned when AsyncRetry is in shutdown.
	Do(ctx context.Context, f RetryableFunc, finish FinishFunc, opts ...Option) error

	// Shutdown gracefully shuts down AsyncRetry without interrupting any active `Do`.
	// Shutdown works by first stopping to accept new `Do` request, and then waiting for all active `Do`'s background goroutines to be finished.
	// Multiple call of Shutdown is OK.
	Shutdown(ctx context.Context) error
}

type asyncRetry struct {
	mu           sync.RWMutex // guards wg and shutdownChan
	wg           sync.WaitGroup
	shutdownChan chan struct{}
}

func NewAsyncRetry() AsyncRetry {
	return &asyncRetry{
		wg:           sync.WaitGroup{},
		shutdownChan: make(chan struct{}),
	}
}

var ErrInShutdown = fmt.Errorf("AsyncRetry is in shutdown")

func (a *asyncRetry) Do(ctx context.Context, f RetryableFunc, finish FinishFunc, opts ...Option) error {
	config := DefaultConfig
	for _, opt := range opts {
		opt(&config)
	}

	a.mu.RLock()
	select {
	case <-a.shutdownChan:
		a.mu.RUnlock()
		return ErrInShutdown
	default:
	}
	a.wg.Add(1) // notice that this line should be in lock so that shutdown would not go ahead
	a.mu.RUnlock()

	go func() {
		defer a.wg.Done() // Done should be called after `finish` returns
		defer func() {
			if recovered := recover(); recovered != nil {
				var err = fmt.Errorf("panicking while AsyncRetry err: %v", recovered)
				finish(err)
			}
		}()
		var err = a.call(ctx, f, &config)
		finish(err)
	}()
	return nil
}

func (a *asyncRetry) call(ctx context.Context, f RetryableFunc, config *Config) error {
	ctx, cancel := context.WithCancel(WithoutCancel(ctx))
	defer cancel()
	noMoreRetryCtx, noMoreRetry := context.WithCancel(config.context)
	defer noMoreRetry()

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-a.shutdownChan:
			noMoreRetry()
			if config.cancelWhenShutdown {
				cancel()
			}
		case <-config.context.Done():
			if config.cancelWhenConfigContextCanceled {
				cancel()
			}
		case <-done: // release resources
		}
	}()

	return retry.Do(
		func() error {
			if config.timeout > 0 {
				timeoutCtx, timeoutCancel := context.WithTimeout(ctx, config.timeout)
				defer timeoutCancel()
				return f(timeoutCtx)
			}
			return f(ctx)
		},
		retry.Attempts(config.attempts),
		retry.OnRetry(retry.OnRetryFunc(config.onRetry)),
		retry.RetryIf(retry.RetryIfFunc(config.retryIf)),
		retry.Context(noMoreRetryCtx),
		retry.Delay(config.delay),
		retry.MaxJitter(config.maxJitter),
	)
}

func (a *asyncRetry) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	select {
	case <-a.shutdownChan: // Already closed.
	default: // Guarded by a.mu
		close(a.shutdownChan)
	}
	a.mu.Unlock()

	ch := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(ch)
	}()

	select {
	case <-ch:
	case <-ctx.Done():
	}
	return ctx.Err()
}

// Unrecoverable wraps error.
func Unrecoverable(err error) error {
	return retry.Unrecoverable(err)
}

// IsRecoverable checks if error is recoverable
func IsRecoverable(err error) bool {
	return retry.IsRecoverable(err)
}
