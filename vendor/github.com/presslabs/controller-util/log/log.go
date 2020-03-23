/*
Copyright 2019 Pressinfra SRL

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
	"io"
	"os"
	"time"

	"github.com/blendle/zapdriver"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	// Log is the base logger used by kubebuilder.  It delegates
	// to another logr.Logger.  You *must* call SetLogger to
	// get any actual logging.
	Log = logf.Log

	// KBLog is a base parent logger.
	KBLog = logf.KBLog

	// SetLogger sets a concrete logging implementation for all deferred Loggers.
	SetLogger = logf.SetLogger
)

// KubeAwareEncoder is a Kubernetes-aware Zap Encoder.
// Instead of trying to force Kubernetes objects to implement
// ObjectMarshaller, we just implement a wrapper around a normal
// ObjectMarshaller that checks for Kubernetes objects.
type KubeAwareEncoder = logf.KubeAwareEncoder

// ZapLogger is a Logger implementation.
// If development is true, a Zap development config will be used
// (stacktraces on warnings, no sampling), otherwise a Zap production
// config will be used (stacktraces on errors, sampling).
func ZapLogger(development bool) logr.Logger {
	return ZapLoggerTo(os.Stderr, development)
}

// ZapLoggerTo returns a new Logger implementation using Zap which logs
// to the given destination, instead of stderr.  It otherise behaves like
// ZapLogger.
func ZapLoggerTo(destWriter io.Writer, development bool) logr.Logger {
	return zapr.NewLogger(RawZapLoggerTo(destWriter, development))
}

// RawZapLoggerTo returns a new zap.Logger configured with KubeAwareEncoder
// which logs to a given destination
func RawZapLoggerTo(destWriter io.Writer, development bool, opts ...zap.Option) *zap.Logger {
	// this basically mimics New<type>Config, but with a custom sink
	sink := zapcore.AddSync(destWriter)

	var enc zapcore.Encoder
	var lvl zap.AtomicLevel
	if development {
		encCfg := zapdriver.NewDevelopmentEncoderConfig()
		enc = zapcore.NewConsoleEncoder(encCfg)
		lvl = zap.NewAtomicLevelAt(zap.DebugLevel)
		opts = append(opts, zap.Development(), zap.AddStacktrace(zap.ErrorLevel))
	} else {
		encCfg := zapdriver.NewProductionEncoderConfig()
		enc = zapcore.NewJSONEncoder(encCfg)
		lvl = zap.NewAtomicLevelAt(zap.InfoLevel)
		opts = append(opts, zap.AddStacktrace(zap.WarnLevel),
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return zapcore.NewSampler(core, time.Second, 100, 100)
			}))
	}
	opts = append(opts, zap.AddCallerSkip(1), zap.ErrorOutput(sink))
	log := zap.New(zapcore.NewCore(&KubeAwareEncoder{Encoder: enc, Verbose: development}, sink, lvl))
	log = log.WithOptions(opts...)

	return log
}
