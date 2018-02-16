package sync

import (
	gsync "sync"
	"sync/atomic"
)

// Once is an object that will perform exactly one action.
type Once struct {
	m    gsync.Mutex
	done int32
}

// Do calls the function f if and only if Do is being called for the
// first time successfully for this instance of Once. In other words, given
// 	var once Once
// if once.Do(f) is called multiple times, f will be invoked until first successful execution.
// A new instance of Once is required for each function to execute.
//
// Do is intended for initialization that must be run exactly once successfully.
//
// Because no call to Do returns until the one call to f returns, if f causes
// Do to be called, it will deadlock.
//
// If f panics, Do considers it to have returned successfully; future calls of Do return
// without calling f.
//
// This is an adaptation from https://golang.org/pkg/sync/#Once
//
func (o *Once) Do(f func() error) {
	if atomic.LoadInt32(&o.done) == 1 {
		return
	}
	// Slow-path.
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.AddInt32(&o.done, 1)
		err := f()
		if err != nil {
			atomic.StoreInt32(&o.done, -1)
		}
	}
}
