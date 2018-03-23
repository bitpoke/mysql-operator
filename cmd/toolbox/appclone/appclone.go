package appclone

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"

	tb "github.com/presslabs/titanium/cmd/toolbox/util"
)

// RunCloneCommand clone the data from source.
func RunCloneCommand(stopCh <-chan struct{}) error {
	glog.Infof("Cloning into node %s", tb.GetHostname())

	// skip cloning if data exists.
	init, err := checkIfWasInit()
	if err != nil {
		return fmt.Errorf("failed to read init file: %s", err)
	} else if init {
		glog.Info("Data exists and is initialized. Skip clonging.")
		return nil
	} else {
		if checkIfDataExists() {
			glog.Infof("Data alerady exists! Remove manualy PVC to cleanup or to reinitialize.")
			return nil
		}
	}

	if tb.NodeRole() == "master" {
		initBucket := tb.GetInitBucket()
		if len(initBucket) == 0 {
			glog.Info("Skip cloning init bucket uri is not set.")
			// let mysqld initialize data dir
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

	return nil
}

func cloneFromBucket(initBucket string) error {
	initBucket = strings.Replace(initBucket, "://", ":", 1)

	glog.Infof("Cloning from bucket: %s", initBucket)

	if _, err := os.Stat(tb.RcloneConfigFile); os.IsNotExist(err) {
		glog.Fatalf("Rclone config file does not exists. err: %s", err)
		return err
	}
	// rclone --config={conf file} cat {bucket uri}
	// writes to stdout the content of the bucket uri
	rclone := exec.Command("rclone", "-vv",
		fmt.Sprintf("--config=%s", tb.RcloneConfigFile), "cat", initBucket)

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

	rclone.Stderr = os.Stderr
	gzip.Stderr = os.Stderr
	xbstream.Stderr = os.Stderr

	if err := rclone.Start(); err != nil {
		return fmt.Errorf("rclone start error: %s", err)
	}

	if err := gzip.Start(); err != nil {
		return fmt.Errorf("gzip start error: %s", err)
	}

	if err := xbstream.Start(); err != nil {
		return fmt.Errorf("xbstream start error: %s", err)
	}

	if err := rclone.Wait(); err != nil {
		return fmt.Errorf("rclone wait error: %s", err)
	}

	if err := gzip.Wait(); err != nil {
		return fmt.Errorf("gzip wait error: %s", err)
	}

	if err := xbstream.Wait(); err != nil {
		return fmt.Errorf("xbstream wait error: %s", err)
	}

	glog.Info("Cloning done successfully.")
	return nil
}

func cloneFromSource(host string) error {
	glog.Infof("Cloning from node: %s", host)
	// ncat --recv-only {host} {port}
	// connects to host and get data from there
	ncat := exec.Command("ncat", "--recv-only", host, tb.BackupPort)

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	xbstream := exec.Command("xbstream", "-x", "-C", tb.DataDir)

	var err error
	if xbstream.Stdin, err = ncat.StdoutPipe(); err != nil {
		return fmt.Errorf("set pipe, error: %s", err)
	}

	if err := ncat.Start(); err != nil {
		return fmt.Errorf("ncat start error: %s", err)
	}

	if err := xbstream.Start(); err != nil {
		return fmt.Errorf("xbstream start error: %s", err)
	}

	if err := ncat.Wait(); err != nil {
		return fmt.Errorf("ncat wait error: %s", err)
	}

	if err := xbstream.Wait(); err != nil {
		return fmt.Errorf("xbstream wait error: %s", err)
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

func checkIfWasInit() (bool, error) {
	_, err := os.Open(fmt.Sprintf("%s/%s/%s.CSV", tb.DataDir, tb.ToolsDbName,
		tb.ToolsInitTableName))

	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// maybe check csv init data and log it

	return true, nil
}
