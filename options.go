package asyncretry

import (
	"context"
	"time"
)

// Option represents an option for async-retry.
type Option func(*Config)

type Config struct {
	timeout                         time.Duration
	attempts                        uint
	delay                           time.Duration
	maxJitter                       time.Duration
	onRetry                         OnRetryFunc
	retryIf                         RetryIfFunc
	context                         context.Context
	cancelWhenShutdown              bool
	cancelWhenConfigContextCanceled bool
}

var DefaultConfig = Config{
	timeout:                         time.Second * 10,
	context:                         context.Background(),
	cancelWhenShutdown:              false,
	cancelWhenConfigContextCanceled: true,
	attempts:                        10,
	onRetry:                         func(n uint, err error) {},
	retryIf:                         IsRecoverable,
	delay:                           time.Millisecond * 100,
	maxJitter:                       time.Millisecond * 100,
}

// Timeout sets timeout for AsyncRetryFunc
// 0 or value less than 0 means no timeout. In most cases, setting timeout is preferable.
func Timeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.timeout = timeout
	}
}

// Context sets context
// If context is canceled, stop next retry
func Context(ctx context.Context) Option {
	return func(c *Config) {
		c.context = ctx
	}
}

// CancelWhenShutdown sets boolean
// If true, cancel the context specified as an argument of AsyncRetryFunc when Shutdown is called
func CancelWhenShutdown(cancel bool) Option {
	return func(c *Config) {
		c.cancelWhenShutdown = cancel
	}
}

// CancelWhenConfigContextCanceled sets boolean
// If true, cancel the context specified as an argument of AsyncRetryFunc when c.context is canceled
func CancelWhenConfigContextCanceled(cancel bool) Option {
	return func(c *Config) {
		c.cancelWhenConfigContextCanceled = cancel
	}
}

// All the options below this line are wrapper of options of github.com/avast/retry-go
// For detail, refer to https://github.com/avast/retry-go/blob/master/options.go

// Attempts set count of retry
// If set 0 or 1, only initial call is permitted
// If set 2, initial call and one retry are permitted
// Note: retry-go supports 0 which means infinity, but we don't support that
func Attempts(attempts uint) Option {
	return func(c *Config) {
		if attempts == 0 {
			attempts = 1
		}
		c.attempts = attempts
	}
}

// OnRetryFunc is a signature of OnRetry function
// n = 0-indexed count of attempts.
type OnRetryFunc func(n uint, err error)

// OnRetry sets OnRetryFunc
func OnRetry(onRetry OnRetryFunc) Option {
	return func(c *Config) {
		c.onRetry = onRetry
	}
}

// RetryIfFunc is s signature of retry if function
type RetryIfFunc func(error) bool

// RetryIf sets RetryIfFunc
// If retryIf returns false, retry will be stopped
func RetryIf(retryIf RetryIfFunc) Option {
	return func(c *Config) {
		c.retryIf = retryIf
	}
}

// Delay sets delay between retry
// Interval between retry equals to the sum of c.delay << n and random number in the half-open interval [0,c.maxJitter)
func Delay(delay time.Duration) Option {
	return func(c *Config) {
		c.delay = delay
	}
}

// MaxJitter sets the maximum random jitter
// See comment of Delay
func MaxJitter(maxJitter time.Duration) Option {
	return func(c *Config) {
		c.maxJitter = maxJitter
	}
}
