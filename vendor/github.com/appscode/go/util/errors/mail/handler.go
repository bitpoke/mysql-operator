package mail

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/appscode/go-notify"
	utilerrors "github.com/appscode/go/util/errors"
	"github.com/pkg/errors"
)

type BodyFunc func(error, errors.StackTrace) (body string, isHTML bool)

// mailer sends email via notifier in case of errors.
type mailer struct {
	notifier notify.ByEmail

	// Body receives the error instance and provide
	// string email body and a boolean value. If true, the boolean value
	// indicates the email is html email
	Body BodyFunc
}

var _ utilerrors.Handler = &mailer{}

func (h *mailer) Handle(err error, st errors.StackTrace) {
	subject := "ERROR - " + err.Error()
	var body string
	var html bool
	if st != nil {
		body, html = h.Body(err, st)
	}
	if html {
		h.notifier.WithSubject(subject).
			WithBody(body).
			SendHtml()
	} else {
		h.notifier.WithSubject(subject).
			WithBody(body).
			Send()
	}
}

func New(notifier notify.ByEmail, fn BodyFunc) utilerrors.Handler {
	handler := &mailer{
		notifier: notifier,
		Body: func(e error, st errors.StackTrace) (string, bool) {
			host, _ := os.Hostname()
			return fmt.Sprintf(defaultEmailTemplate,
				time.Now().String(),
				e.Error(),
				fmt.Sprintf("%+v", st),
				host,
				runtime.GOOS,
			), true
		},
	}
	if fn != nil {
		handler.Body = fn
	}
	return handler
}
