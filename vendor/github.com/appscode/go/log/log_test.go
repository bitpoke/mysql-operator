package log_test

import (
	"flag"
	"testing"

	alog "github.com/appscode/go/log"
	"github.com/golang/glog"
)

func init() {
	// flag.Set("logtostderr", "true")
	flag.Set("v", "5")
}

var data = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}

func BenchmarkAppsCodeLog(b *testing.B) {
	for n := 0; n < b.N; n++ {
		alog.Infoln(data)
	}
}

func BenchmarkGLog(b *testing.B) {
	for n := 0; n < b.N; n++ {
		glog.Infoln(data)
	}
}

func TestInfof(t *testing.T) {
	flag.Set("stderrthreshold", "INFO")
	// flag.Set("logtostderr", "true")
	alog.Infof("id: %s data: %s", "hello", "world")
}
