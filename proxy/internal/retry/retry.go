package retry

import (
	"math/rand"
	"time"
)

type Config struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

func ExponentialBackoff(cfg Config, fn func() error) error {
	var err error
	for i := 0; i <= cfg.MaxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if i == cfg.MaxRetries {
			return err
		}
		delay := cfg.BaseDelay * (1 << i)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		jitter := time.Duration(rand.Int63n(int64(delay)))
		time.Sleep(delay + jitter)
	}
	return err
}
