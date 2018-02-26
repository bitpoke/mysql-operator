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
			sourceHost = tb.GetMasterService()
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

	return xtrabackupPreperData()
}

const (
	mysqlDataDir     = "/var/lib/mysql"
	rcloneConfigFile = "/etc/mysql/rclone.conf"
)

func cloneFromBucket() error {
	if _, err := os.Stat(rcloneConfigFile); os.IsNotExist(err) {
		glog.Fatalf("Rclone config file does not exists. err: %s", err)
		return err
	}
	initBucket := tb.GetInitBucket()
	// rclone --config={conf file} cat {bucket uri}
	// writes to stdout the content of the bucket uri
	rclone := exec.Command("rclone",
		fmt.Sprintf("--config=%s", rcloneConfigFile), "cat", initBucket)

	// gzip reads from stdin decompress and then writes to stdout
	gzip := exec.Command("gzip", "-d")

	// xbstream -x -C {mysql data target dir}
	// extracts files from stdin (-x) and writes them to mysql
	// data target dir
	xbstream := exec.Command("xbstream", "-x", "-C", mysqlDataDir)

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
	xbstream := exec.Command("xbstream", "-x", "-C", mysqlDataDir)

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
	xtbkCmd := exec.Command("xtrabackup", "--prepare",
		fmt.Sprintf("--target-dir=%s", mysqlDataDir),
		fmt.Sprintf("--user=%s", replUser), fmt.Sprintf("--password=%s", replPass))

	return xtbkCmd.Run()
}
