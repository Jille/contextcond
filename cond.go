// Package contextcond provides a sync.Cond that has a WaitContext method.
//
// Internally it's implemented using channels and mutexes.
package contextcond

import (
	"context"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Cond is like sync.Cond with a WaitContext method.
// The zero value is valid except that L must be set.
type Cond struct {
	// L is held while observing or changing the condition
	L sync.Locker

	// mtx guards ch and waiters.
	mtx     sync.Mutex
	ch      chan struct{}
	waiters int

	checker copyChecker
}

// NewCond returns a new Cond with Locker l.
func NewCond(l sync.Locker) *Cond {
	return &Cond{L: l}
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L during the call.
func (c *Cond) Broadcast() {
	c.checker.check()
	c.mtx.Lock()
	if c.waiters > 0 {
		close(c.ch)
		c.ch = make(chan struct{}, 1)
	}
	c.mtx.Unlock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L during the call.
//
// Signal() does not affect goroutine scheduling priority; if other goroutines are attempting to lock c.L, they may be awoken before a "waiting" goroutine.
func (c *Cond) Signal() {
	c.checker.check()
	c.mtx.Lock()
	if c.waiters > 0 {
		select {
		case c.ch <- struct{}{}:
		default:
		}
	}
	c.mtx.Unlock()
}

// Wait atomically unlocks c.L and suspends execution of the calling goroutine. After later resuming execution, Wait locks c.L before returning. Unlike in other systems, Wait cannot return unless awoken by Broadcast or Signal.
// Because c.L is not locked while Wait is waiting, the caller typically cannot assume that the condition is true when Wait returns. Instead, the caller should Wait in a loop.
func (c *Cond) Wait() {
	c.WaitContext(context.Background())
}

// WaitContext is like wait, but returns a non-nil error iff the context was cancelled.
func (c *Cond) WaitContext(ctx context.Context) error {
	c.checker.check()
	c.mtx.Lock()
	if c.ch == nil {
		c.ch = make(chan struct{}, 1)
	}
	ch := c.ch
	c.waiters++
	c.mtx.Unlock()

	c.L.Unlock()

	var err error
	select {
	case <-ch:
		err = nil
	case <-ctx.Done():
		err = ctx.Err()
	}

	c.mtx.Lock()
	c.waiters--
	if c.waiters == 0 {
		select {
		case <-c.ch:
		default:
		}
	}
	c.mtx.Unlock()

	c.L.Lock()

	return err
}

// copyChecker holds back pointer to itself to detect object copying.
type copyChecker uintptr

func (c *copyChecker) check() {
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) {
		panic("contextcond.Cond is copied")
	}
}
