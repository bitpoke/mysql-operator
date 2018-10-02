package errors

import (
	"sync"

	"github.com/pkg/errors"
)

// Handler is a error-handling middleware that receives an error and executes handlers in order.
type Handler interface {
	Handle(err error, st errors.StackTrace)
}

var (
	handlers []Handler
	lock     sync.RWMutex
)

func Register(h Handler) {
	lock.Lock()
	defer lock.Unlock()

	handlers = append(handlers, h)
}

func HandleError(err error) {
	if err == nil {
		return
	}

	lock.RLock()
	defer lock.RUnlock()

	var st errors.StackTrace
	cause, ok := errors.Cause(err).(stackTracer)
	if ok {
		st = cause.StackTrace()
	}

	for _, handler := range handlers {
		go func() {
			handler.Handle(err, st)
		}()
	}
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}
