package appclone

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/golang/glog"

	tb "github.com/presslabs/titanium/cmd/toolbox/util"
)

// RunCloneCommand clone the data from source.
func RunCloneCommand(stopCh <-chan struct{}) error {
	glog.Infof("Cloning into node %s", tb.GetHostname())

	// skip cloning if data exists.
	if _, err := os.Stat(tb.CheckDataFile); !os.IsNotExist(err) {
		glog.Info("Data exists, Skip clonging.")
		return nil
	}

	if tb.GetServerId() == 100 {
		// cloning for the first master
		initBucket := tb.GetInitBucket()
		if len(initBucket) == 0 {
			glog.Info("Skip cloning init bucket uri is not set.")
			return nil
		}
		err := cloneFromBucket(initBucket)
		if err != nil {
			return fmt.Errorf("failed to clone from bucket, err: %s", err)
		}
	} else {
		// clonging from prior node
		sourceHost := tb.GetHostFor(tb.GetServerId() - 1)
		err := cloneFromSource(sourceHost)
		if err != nil {
			return fmt.Errorf("faild to clone from %s, err: %s", sourceHost, err)
		}
	}

	// prepare backup
	if err := xtrabackupPreperData(); err != nil {
		return err
	}

	// create check file.
	if _, err := os.Create(tb.CheckDataFile); err != nil {
		glog.Fatalf("Failed to create check file: %s, err: %s", tb.CheckDataFile, err)
		return err
	}

	return nil
}

const (
	// rcloneConfigFile represents the path to the file that contains rclon
	// configs. This path should be the same as defined in docker entrypoint
	// script from toolbox/docker-entrypoint.sh. /etc/rclone.conf
	rcloneConfigFile = "/etc/rclone.conf"
)

func cloneFromBucket(initBucket string) error {

	if _, err := os.Stat(rcloneConfigFile); os.IsNotExist(err) {
		glog.Fatalf("Rclone config file does not exists. err: %s", err)
		return err
	}
	// rclone --config={conf file} cat {bucket uri}
	// writes to stdout the content of the bucket uri
	rclone := exec.Command("rclone",
		fmt.Sprintf("--config=%s", rcloneConfigFile), "cat", initBucket)

	// gzip reads from stdin decompress and then writes to stdout
	gzip := exec.Command("gzip", "-d")

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	xbstream := exec.Command("xbstream", "-x", "-C", tb.DataDir)

	var err error
	// rclone | gzip | xbstream
	if gzip.Stdin, err = rclone.StdoutPipe(); err != nil {
		return err
	}

	if xbstream.Stdin, err = gzip.StdoutPipe(); err != nil {
		return err
	}

	var out bytes.Buffer
	xbstream.Stdout = &out
	defer io.Copy(os.Stdout, &out)

	if err := rclone.Start(); err != nil {
		return err
	}

	if err := gzip.Start(); err != nil {
		return err
	}

	if err := xbstream.Start(); err != nil {
		return err
	}

	if err := rclone.Wait(); err != nil {
		return err
	}

	if err := gzip.Wait(); err != nil {
		return err
	}

	if err := xbstream.Wait(); err != nil {
		return err
	}

	return nil
}

func cloneFromSource(host string) error {
	// ncat --recv-only {host} {port}
	// connects to host and get data from there
	ncat := exec.Command("ncat", "--recv-only", host, tb.BackupPort)

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	xbstream := exec.Command("xbstream", "-x", "-C", tb.DataDir)

	var err error
	if xbstream.Stdin, err = ncat.StdoutPipe(); err != nil {
		return err
	}

	if err := ncat.Start(); err != nil {
		return err
	}

	if err := xbstream.Start(); err != nil {
		return err
	}

	if err := ncat.Wait(); err != nil {
		return err
	}

	if err := xbstream.Wait(); err != nil {
		return err
	}

	return nil
}

func xtrabackupPreperData() error {
	replUser := tb.GetReplUser()
	replPass := tb.GetReplPass()

	// TODO: remove user and password for here, not needed.
	xtbkCmd := exec.Command("xtrabackup", "--prepare",
		fmt.Sprintf("--target-dir=%s", tb.DataDir),
		fmt.Sprintf("--user=%s", replUser), fmt.Sprintf("--password=%s", replPass))

	return xtbkCmd.Run()
}
