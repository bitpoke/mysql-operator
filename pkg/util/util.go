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

package util

import (
	"crypto/rand"
	"encoding/base64"
	mrand "math/rand"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	batch "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/presslabs/mysql-operator/pkg/util/constants"
)

func init() {
	mrand.Seed(time.Now().UnixNano())
}

func GetPodNamespace() string {
	ns := os.Getenv(constants.EnvOperatorPodNamespace)
	if len(ns) == 0 {
		logrus.Fatalf("must set env (%s)", constants.EnvOperatorPodNamespace)
	}
	return ns
}

func GetPodName() string {
	name := os.Getenv(constants.EnvOperatorPodName)
	if len(name) == 0 {
		logrus.Fatalf("must set env (%s)", constants.EnvOperatorPodName)
	}
	return name
}

func GetPodHostName() string {
	host, err := os.Hostname()
	if err != nil {
		logrus.Fatalf("failed to get hostname: %v", err)
	}
	return host
}

func CreateEventRecorder(kubecli kubernetes.Interface, name, namespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: v1core.New(kubecli.Core().RESTClient()).Events(namespace),
	})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: name})
}

func RandomString(length int) string {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func randStringFrom(chars []rune, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = chars[mrand.Intn(len(chars))]
	}
	return string(b)
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringUser(n int) string {
	return randStringFrom(letterRunes, n)
}

var lowerLetters = []rune("abcdefghijklmnopqrstuvwxyz")

func RandStringLowerLetters(n int) string {
	return randStringFrom(lowerLetters, n)
}

// JobConditionIndex
func JobConditionIndex(ty batch.JobConditionType, cs []batch.JobCondition) (int, bool) {
	for i, cond := range cs {
		if cond.Type == ty {
			return i, true
		}
	}
	return 0, false
}
