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

package testutil

import (
	"io"
	"time"

	// loggging
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	drainTimeout = 200 * time.Millisecond
)

// DrainChan drains the request chan time for drainTimeout
func DrainChan(requests <-chan reconcile.Request) {
	for {
		select {
		case <-requests:
			continue
		case <-time.After(drainTimeout):
			return
		}
	}
}

// NewTestLogger returns a logger good for tests
func NewTestLogger(w io.Writer, options ...zap.Option) logr.Logger {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	sink := zapcore.AddSync(w)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), sink, zap.DebugLevel)
	return zapr.NewLogger(zap.New(core).WithOptions(options...))
}
