package appschedulebackup

import (
	"github.com/golang/glog"
)

func RunCommand(stopCh <-chan struct{}, cluster string) error {
	glog.Infof("Schedule backup for cluster: %s", cluster)
	// TODO: ...

	return nil
}
