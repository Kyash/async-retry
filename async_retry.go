package asyncretry

import (
	"context"
	"fmt"
	"sync"

	"github.com/avast/retry-go/v4"
)

type AsyncRetryFunc func(ctx context.Context) error

type AsyncRetry interface {
	// Do calls f and retry if necessary.
	// In most cases, you should call Do in a new goroutine.
	Do(ctx context.Context, f AsyncRetryFunc, opts ...Option) error
	// Shutdown shutdowns gracefully
	Shutdown(ctx context.Context) error
}

type asyncRetry struct {
	// FIXME use RWMutex instead
	mu           sync.Mutex // guards wg and shutdownChan
	wg           sync.WaitGroup
	shutdownChan chan struct{}
}

func NewAsyncRetry() AsyncRetry {
	return &asyncRetry{
		wg:           sync.WaitGroup{},
		shutdownChan: make(chan struct{}),
	}
}

var InShutdownErr = fmt.Errorf("AsyncRetry is in shutdown")

func (a *asyncRetry) Do(ctx context.Context, f AsyncRetryFunc, opts ...Option) (retErr error) {
	a.mu.Lock()
	select {
	case <-a.shutdownChan:
		return InShutdownErr
	default:
	}
	a.wg.Add(1)
	a.mu.Unlock()
	defer a.wg.Done()

	config := DefaultConfig
	for _, opt := range opts {
		opt(&config)
	}

	defer func() {
		if err := recover(); err != nil {
			retErr = fmt.Errorf("panicking while AsyncRetry err: %v", err)
		}
	}()

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
		// release resources
		case <-done:
		}
	}()

	return retry.Do(
		func() error {
			if config.timeout > 0 {
				var timeoutCancel context.CancelFunc
				ctx, timeoutCancel = context.WithTimeout(ctx, config.timeout)
				defer timeoutCancel()
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
	case <-a.shutdownChan:
		// Already closed.
	default:
		// Guarded by a.mu
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
