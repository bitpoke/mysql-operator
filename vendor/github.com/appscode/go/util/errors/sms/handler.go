package sms

import (
	"github.com/appscode/go-notify"
	utilerrors "github.com/appscode/go/util/errors"
	"github.com/pkg/errors"
)

// messenger sends chat via notifier in case of errors.
type messenger struct {
	notifier notify.BySMS
}

var _ utilerrors.Handler = &messenger{}

func New(notifier notify.BySMS) utilerrors.Handler {
	return &messenger{notifier}
}

func (h *messenger) Handle(err error, st errors.StackTrace) {
	h.notifier.WithBody(err.Error()).Send()
}
