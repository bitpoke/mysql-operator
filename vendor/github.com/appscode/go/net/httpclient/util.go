package httpclient

import (
	"time"

	"github.com/cenkalti/backoff"
)

// Default values for ExponentialBackOff.
const (
	DefaultInitialInterval     = 50 * time.Millisecond
	DefaultRandomizationFactor = 0.5
	DefaultMultiplier          = 1.5
	DefaultMaxInterval         = 5 * time.Second
	DefaultMaxElapsedTime      = 60 * time.Second
)

// NewExponentialBackOff creates an instance of ExponentialBackOff using default values.
func NewExponentialBackOff() *backoff.ExponentialBackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     DefaultInitialInterval,
		RandomizationFactor: DefaultRandomizationFactor,
		Multiplier:          DefaultMultiplier,
		MaxInterval:         DefaultMaxInterval,
		MaxElapsedTime:      DefaultMaxElapsedTime,
		Clock:               backoff.SystemClock,
	}
	if b.RandomizationFactor < 0 {
		b.RandomizationFactor = 0
	} else if b.RandomizationFactor > 1 {
		b.RandomizationFactor = 1
	}
	b.Reset()
	return b
}
