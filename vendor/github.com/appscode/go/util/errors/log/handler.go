package log

import (
	utilerrors "github.com/appscode/go/util/errors"
	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type logger struct{}

var _ utilerrors.Handler = &logger{}

func New() utilerrors.Handler {
	return &logger{}
}

func (logger) Handle(err error, st errors.StackTrace) {
	if st != nil {
		glog.Errorln(err)
		glog.Errorf("%+v", st)
	}
}
