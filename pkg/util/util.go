package util

import (
	"math/rand"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/presslabs/titanium/pkg/util/constants"
)

var random *rand.Rand // Rand for this package.

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
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
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[random.Intn(len(chars))]
	}
	return string(result)
}
