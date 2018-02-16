package mail

import (
	"fmt"
	"os"
	"runtime"
	"time"

	notify "github.com/appscode/go-notify"
	"github.com/appscode/go/context"
	"github.com/appscode/go/errors"
)

// Email handler sends email via mailgun in case of errors
// additional mail handler can be implemented and can plugin
// in Handlers.
type EmailHandler struct {
	mailer notify.ByEmail

	// Body receives the error instance and provide
	// string email body and a boolean value. If true, the boolean value
	// indicates the email is html email
	Body func(error) (body string, isHTML bool)
}

func (h *EmailHandler) Handle(error error) {
	if err := errors.FromErr(error); err != nil {
		body, html := h.Body(error)
		if body != "" {
			subject := "ERROR - "
			if err.Cause() != nil {
				subject += err.Cause().Error()
			} else if len(err.Message()) > 0 {
				subject += err.Message()
			}
			if html {
				h.mailer.WithSubject(subject).
					WithBody(body).
					SendHtml()
			} else {
				h.mailer.WithSubject(subject).
					WithBody(body).
					Send()
			}
		}
	}
}

func NewEmailhandler(mailer notify.ByEmail) *EmailHandler {
	return &EmailHandler{
		mailer: mailer,
		Body:   defaultBodyFunction,
	}
}

var defaultBodyFunction = func(e error) (string, bool) {
	err := errors.FromErr(e)
	if err != nil {
		host, _ := os.Hostname()
		errCause := ""
		if cause := err.Cause(); cause != nil {
			errCause = cause.Error()
		}
		contextValues := ""
		if c := context.ID(err.Context()); c != "" {
			contextValues = c
		}

		return fmt.Sprintf(defaultEmailTemplate,
			time.Now().String(),
			errCause,
			err.Message(),
			contextValues,
			err.Trace().String(),
			host,
			runtime.GOOS,
		), true
	}
	return "", false
}
