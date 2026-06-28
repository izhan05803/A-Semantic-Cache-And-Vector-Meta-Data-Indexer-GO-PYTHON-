package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

var ErrOpen = errors.New("circuit breaker: open")

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

type Breaker struct {
	mu               sync.Mutex
	state            State
	failureCount     uint64
	failureThreshold uint64
	recoveryTimeout  time.Duration
	lastFailure      time.Time
}

func New(failureThreshold uint64, recoveryTimeout time.Duration) *Breaker {
	return &Breaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()
	switch b.state {
	case StateOpen:
		if time.Since(b.lastFailure) > b.recoveryTimeout {
			b.state = StateHalfOpen
		} else {
			b.mu.Unlock()
			return ErrOpen
		}
	case StateHalfOpen:
	default:
	}
	b.mu.Unlock()

	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.failureCount++
		b.lastFailure = time.Now()
		if b.failureCount >= b.failureThreshold {
			b.state = StateOpen
		}
	} else {
		b.failureCount = 0
		if b.state == StateHalfOpen {
			b.state = StateClosed
		}
	}
	return err
}
