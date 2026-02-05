package workflow

import (
	"context"
)

// Semaphore limits concurrent operations
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a new semaphore with the given capacity
func NewSemaphore(max int) *Semaphore {
	if max <= 0 {
		max = 1
	}
	return &Semaphore{
		ch: make(chan struct{}, max),
	}
}

// Acquire acquires a semaphore slot, blocking until one is available
// Returns an error if the context is cancelled
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire attempts to acquire a semaphore slot without blocking
// Returns true if successful, false otherwise
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release releases a semaphore slot
func (s *Semaphore) Release() {
	select {
	case <-s.ch:
	default:
		// Already empty, this shouldn't happen but handle gracefully
	}
}

// Available returns the number of available slots
func (s *Semaphore) Available() int {
	return cap(s.ch) - len(s.ch)
}

// Capacity returns the maximum capacity
func (s *Semaphore) Capacity() int {
	return cap(s.ch)
}

// InUse returns the number of slots currently in use
func (s *Semaphore) InUse() int {
	return len(s.ch)
}
