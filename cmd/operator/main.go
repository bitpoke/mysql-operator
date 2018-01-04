package main

import (
	"flag"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/presslabs/titanium/pkg/controller"
	"github.com/presslabs/titanium/pkg/util"
	"github.com/presslabs/titanium/pkg/util/k8sutil"
)

var (
	namespace string
	name      string
)

func init() {
	flag.Parse()
}

func main() {
	namespace = util.GetPodNamespace()
	name = util.GetPodName()

	kubecli := k8sutil.MustNewKubeClient()
	// TODO: handle signals SIGINT, SIGTERM
	// add promithius endpoints...

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          getResourceLock(kubecli),
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				logrus.Fatalf("leader election lost")
			},
		},
	})

	panic("unreachable")
}

func run(stop <-chan struct{}) {
	cfg := newControllerConfig()
	c := controller.New(cfg)
	err := c.Start()
	logrus.Fatalf("controller Start() failed: %v", err)
}

func getResourceLock(kubecli kubernetes.Interface) resourcelock.Interface {
	rl, err := resourcelock.New(resourcelock.EndpointsResourceLock,
		namespace,
		"mysql-operator-titanium",
		kubecli.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      util.GetPodHostName(),
			EventRecorder: util.CreateEventRecorder(kubecli, name, namespace),
		})
	if err != nil {
		logrus.Fatalf("error creating lock: %v", err)
	}
	return rl
}

func newControllerConfig() controller.Config {
	kubecli := k8sutil.MustNewKubeClient()

	serviceAccount, err := getMyPodServiceAccount(kubecli)
	if err != nil {
		logrus.Fatalf("fail to get my pod's service account: %v", err)
	}

	cfg := controller.Config{
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
		KubeCli:        kubecli,
		KubeExtCli:     k8sutil.MustNewKubeExtClient(),
	}

	return cfg
}

func getMyPodServiceAccount(kubecli kubernetes.Interface) (string, error) {
	var sa string
	pod, err := kubecli.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("fail to get operator pod (%s): %v", name, err)
		return sa, nil
	}
	sa = pod.Spec.ServiceAccountName
	return sa, nil
}
