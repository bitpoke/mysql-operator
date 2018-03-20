package apptakebackup

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/golang/glog"

	tb "github.com/presslabs/titanium/cmd/toolbox/util"
)

func RunTakeBackupCommand(stopCh <-chan struct{}, srcHost, destBucket string) error {
	glog.Infof("Take backup from '%s' to bucket '%s' started...", srcHost, destBucket)
	return pushBackupFromTo(srcHost, destBucket)
}

func pushBackupFromTo(srcHost, destBucket string) error {
	// TODO: document each func
	ncat := exec.Command("ncat", "--recv-only", srcHost, tb.BackupPort)

	gzip := exec.Command("gzip", "-c")

	rclone := exec.Command("rclone",
		fmt.Sprintf("--config=%s", tb.RcloneConfigFile), "rcat", destBucket)

	ncat.Stderr = os.Stderr
	gzip.Stderr = os.Stderr
	rclone.Stderr = os.Stderr

	var err error
	// ncat | gzip | rclone
	if gzip.Stdin, err = ncat.StdoutPipe(); err != nil {
		return err
	}

	if rclone.Stdin, err = gzip.StdoutPipe(); err != nil {
		return err
	}

	if err := ncat.Start(); err != nil {
		return fmt.Errorf("ncat start error: %s", err)
	}

	if err := gzip.Start(); err != nil {
		return fmt.Errorf("gzip start error: %s", err)
	}

	if err := rclone.Start(); err != nil {
		return fmt.Errorf("rclone start error: %s", err)
	}

	glog.V(2).Info("Wait for ncat to finish.")
	if err := ncat.Wait(); err != nil {
		return fmt.Errorf("ncat wait error: %s", err)
	}

	glog.V(2).Info("Wait for gzip to finish.")
	if err := gzip.Wait(); err != nil {
		return fmt.Errorf("gzip wait error: %s", err)
	}

	glog.V(2).Info("Wait for rclone to finish.")
	if err := rclone.Wait(); err != nil {
		return fmt.Errorf("rclone wait error: %s", err)
	}

	return nil
}
