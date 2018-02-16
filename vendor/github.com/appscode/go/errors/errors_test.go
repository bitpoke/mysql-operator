package errors_test

import (
	gtx "context"
	"log"
	"testing"

	"github.com/appscode/go/context"
	"github.com/appscode/go/errors"
)

func TestNew(t *testing.T) {
	err := errors.New().WithMessage("hello-world").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}
}

func TestErrOverlaps(t *testing.T) {
	err := errors.New("this-is-internal", "this-is-also-internal").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	errOverlaps := errors.New().WithCause(err).Err()

	parsedErrOverlaps := errors.FromErr(errOverlaps)
	log.Println("got messages", parsedErrOverlaps.Message())
	log.Println("got error", errOverlaps.Error())
}

type fakeContext string

func (fakeContext) String() string {
	return "fake-context-values"
}

func TestErrWithContext(t *testing.T) {
	err := errors.New().WithContext(context.WithID(gtx.TODO(), "fake-context-values")).WithMessage("hello-world").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}

	parsedErr := errors.FromErr(err)
	if val := context.ID(parsedErr.Context()); val == "" {
		t.Error("expected value fond empty")
	}
	log.Println(context.ID(parsedErr.Context()))
}

func TestStackTrace(t *testing.T) {
	err := errors.New().WithContext(context.WithID(gtx.TODO(), "fake-context-values")).WithMessage("hello-world").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}
	parsedErr := errors.FromErr(err)
	if parsedErr.TraceString() == "" {
		t.Error("expected values got empty")
	}
	log.Println(parsedErr.TraceString())
}

func TestMessagef(t *testing.T) {
	err := errors.Newf("foo-%s", "bar").Err()
	if err == nil {
		t.Error("expected not nil, got nil")
	}
	parsedErr := errors.FromErr(err)
	if parsedErr.Message() != "foo-bar" {
		t.Error("expected values got empty")
	}
	log.Println(parsedErr.Message())
}
