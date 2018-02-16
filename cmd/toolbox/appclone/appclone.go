package appclone

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	tb "github.com/presslabs/titanium/cmd/toolbox/util"
)

// RunCloneCommand clone the data from source.
func RunCloneCommand(stopCh <-chan struct{}) error {
	glog.Infof("Cloning into node %s", tb.GetHostname())

	if tb.GetServerId() == 0 {
		// cloning for the first master
		err := cloneFromBucket()
		if err != nil {
			return fmt.Errorf("failed to clone from bucket, err: %s", err)
		}
	} else {
		// clonging for slaves or comasters

		var sourceHost string
		if tb.GetServerId() <= 100 {
			sourceHost = os.Getenv("MASTER_SERVICE_NAME")
			if len(sourceHost) == 0 {
				return fmt.Errorf("failed to clone on the first node from slaves because " +
					" MASTER_SERVICE_NAME env var is not seted")
			}
		} else {
			sourceHost = tb.GetHostFor(tb.GetServerId() - 1)
		}
		err := cloneFromSource(sourceHost)
		if err != nil {
			return fmt.Errorf("faild to clone from %s, err: %s", sourceHost, err)
		}
	}

	return nil
}

func cloneFromBucket() error {
	return nil
}

func cloneFromSource(host string) error {
	return nil
}
