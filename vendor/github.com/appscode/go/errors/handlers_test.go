package errors

import (
	"log"
	"testing"
	"time"
)

// Important: Those test may fail while run in bundle. Runs tests One by
// one with go test -run exp

func TestDefaultHandler(t *testing.T) {
	Handlers.Add(fakeHandler(""))

	counter = 0
	err := New().WithMessage("internal errors").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	time.Sleep(time.Second)
	if counter != 1 {
		t.Error("counter not 1", "handler not called")
	}
	Handlers = newErrorHandlers()
}

func TestInstanceHandler(t *testing.T) {
	counter = 0
	err := New().WithMessage("internal errors").WithHandler(fakeHandler("")).Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	time.Sleep(time.Second)
	if counter != 1 {
		t.Error("counter not 1", "handler not called")
	}
}

func TestHandlers(t *testing.T) {
	Handlers.Add(fakeHandler(""))

	counter = 0
	err := New().WithMessage("internal errors").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	time.Sleep(time.Second)
	if counter != 1 {
		t.Error("counter not 1", "handler not called")
	}

	counter = 0
	err = New().WithMessage("internal errors").WithHandler(fakeHandler("")).Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	time.Sleep(time.Second)
	if counter != 2 {
		t.Error("counter not 1", "handler not called")
	}
	Handlers = newErrorHandlers()
}

func TestHandlersWithPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error("error failed to recover")
		}
	}()
	Handlers.Add(fakeHandler(""))
	counter = 0
	err := New().WithMessage("internal errors").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	time.Sleep(time.Second)
	if counter != 1 {
		t.Error("counter not 1", "handler not called")
	}

	counter = 0
	err = New().WithMessage("internal errors").WithHandler(fakePanicHandler("")).Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	time.Sleep(time.Second)
	if counter != 1 {
		t.Error("counter not 1", "handler not called")
	}
	Handlers = newErrorHandlers()
}

type fakeHandler string

var counter = 0

func (fakeHandler) Handle(e error) {
	err := FromErr(e)
	if err == nil {
		log.Println("expected not nil, got nil")
	}
	counter++
}

type fakePanicHandler string

func (fakePanicHandler) Handle(e error) {
	panic("run panic handlers")
}
