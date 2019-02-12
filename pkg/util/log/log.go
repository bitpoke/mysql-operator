/*
Copyright 2018 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package log

import (
	"flag"
	"log"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog"
)

var debug = false
var logLevel = 0

func init() {
	flag.BoolVar(&debug, "debug", false, "Set logger in debug mode")
	flag.IntVar(&logLevel, "v", 0, "Set verbosity level")

	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	klogFlags.Set("logtostderr", "true")      // nolint: errcheck
	klogFlags.Set("alsologtostderr", "false") // nolint: errcheck
}

// ZapLogger returns a configured logged based on command line flags -v and --debug
func ZapLogger() logr.Logger {
	cfg := zap.NewProductionConfig()
	maxLevel := 1
	if debug {
		cfg = zap.NewDevelopmentConfig()
		maxLevel = 100
	}

	//cfg.DisableStacktrace = true

	zapLog, err := cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		log.Fatalf("logger building error: %v ", err) // who watches the watchmen?
	}

	// get the v value from flags and then set the logger value
	level := zapcore.WarnLevel
	if logLevel > 0 {
		if logLevel > maxLevel {
			logLevel = maxLevel
		}
		level = zapcore.Level(-1 * logLevel)
	}

	// set debugger level
	cfg.Level.SetLevel(level)

	// the AtomicLevel can be used to dynamically adjust the logging level
	// there's a helper function (zap.LevelFlag) to add a flag for adjusting it too
	return zapr.NewLogger(zapLog)
}
