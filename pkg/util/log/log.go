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
	"strconv"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var debug = false

func init() {
	flag.BoolVar(&debug, "debug", false, "set logger in debug mode")
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
	if strLevel, err := strconv.Atoi(flag.Lookup("v").Value.String()); err == nil {
		if strLevel > maxLevel {
			strLevel = maxLevel
		}
		level = zapcore.Level(-1 * strLevel)
	}

	// set debugger level
	cfg.Level.SetLevel(level)

	// the AtomicLevel can be used to dynamically adjust the logging level
	// there's a helper function (zap.LevelFlag) to add a flag for adjusting it too
	return zapr.NewLogger(zapLog)
}
