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

// Cond is like sync.Cond with a WaitContext method. Unlike sync.Cond, Cond must be initialized with NewCond.
type Cond struct {
	// L is held while observing or changing the condition
	L sync.Locker

	// mtxCh acts like a mutex and guards ch and waiters.
	mtxCh   chan struct{}
	ch      chan struct{}
	waiters int

	delegateWaiterLowering chan struct{}

	checker copyChecker
}

// NewCond returns a new Cond with Locker l.
func NewCond(l sync.Locker) *Cond {
	return &Cond{
		L:                      l,
		mtxCh:                  make(chan struct{}, 1),
		ch:                     make(chan struct{}),
		delegateWaiterLowering: make(chan struct{}),
	}
}

func (c *Cond) checks() {
	if c.mtxCh == nil {
		panic("contextcond.Cond must be initialized with NewCond")
	}
	c.checker.check()
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L during the call.
func (c *Cond) Broadcast() {
	c.checks()
	c.mtxCh <- struct{}{} // lock
	if c.waiters > 0 {
		close(c.ch)
		c.ch = make(chan struct{})
	}
	<-c.mtxCh // unlock
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L during the call.
//
// Signal() does not affect goroutine scheduling priority; if other goroutines are attempting to lock c.L, they may be awoken before a "waiting" goroutine.
func (c *Cond) Signal() {
	c.checks()
	c.mtxCh <- struct{}{} // lock
	for c.waiters > 0 {
		select {
		case c.ch <- struct{}{}:
			// We awoke a waiter. Let them have the lock so they can lower their waiter count. We don't need it anymore anyway.
			return
		case <-c.delegateWaiterLowering:
			// A waiter was awoken by something other than us. They need to lower c.waiters but can't get the lock because we hold it.
			// We'll lower the waiter count for them.
			c.waiters--
		}
	}
	<-c.mtxCh // unlock
}

// Wait atomically unlocks c.L and suspends execution of the calling goroutine. After later resuming execution, Wait locks c.L before returning. Unlike in other systems, Wait cannot return unless awoken by Broadcast or Signal.
// Because c.L is not locked while Wait is waiting, the caller typically cannot assume that the condition is true when Wait returns. Instead, the caller should Wait in a loop.
func (c *Cond) Wait() {
	c.WaitContext(context.Background())
}

// WaitContext is like Wait, but aborts if the given context is cancelled.
// A non-nil error is returned iff the context was cancelled.
// The caller should hold c.L, which is dropped and reacquired during WaitContext. When this function returns it always holds c.L.
func (c *Cond) WaitContext(ctx context.Context) error {
	c.checks()
	c.mtxCh <- struct{}{} // lock
	ch := c.ch
	c.waiters++
	<-c.mtxCh // unlock

	c.L.Unlock()

	var err error
	var signalled bool
	select {
	case _, signalled = <-ch:
		err = nil
	case <-ctx.Done():
		err = ctx.Err()
	}

	// If we were signalled, the Signal() function won't unlock so we now have the lock.
	loweringDelegated := false
	if !signalled {
		select {
		case c.mtxCh <- struct{}{}: // lock
		case c.delegateWaiterLowering <- struct{}{}:
			// Signal() holds the lock and is kind enough to lower lower c.waiters for us.
			// We couldn't just grab the lock ourselves, because we might've been the last waiter and Signal() is blocking on writing to c.ch while holding the lock.
			loweringDelegated = true
		}
	}

	if !loweringDelegated {
		c.waiters--
		<-c.mtxCh // unlock
	}

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
