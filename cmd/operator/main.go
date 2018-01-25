package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/presslabs/titanium/cmd/operator/options"
	controllerpkg "github.com/presslabs/titanium/pkg/controller"
	clientset "github.com/presslabs/titanium/pkg/generated/clientset/versioned"
	informers "github.com/presslabs/titanium/pkg/generated/informers/externalversions"
	"github.com/presslabs/titanium/pkg/util"
	"github.com/presslabs/titanium/pkg/util/k8sutil"

	// Add here all controllers
	_ "github.com/presslabs/titanium/pkg/controller/clustercontroller"
)

var (
	opt                  *options.ControllerOptions
	onlyOneSignalHandler = make(chan struct{})
	shutdownSignals      = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func init() {
	opt = options.NewControllerOptions()
	opt.AddFlags()
	flag.Parse()
	err := opt.Validate()
	if err != nil {
		logrus.Fatalf("Config validation error: %v", err)
	}
}

func main() {
	stopCh := setupSignalHandler()

	ctx := newControllerContext()
	run := func(_ <-chan struct{}) {
		// build controllers map
		var controllers = make(map[string]controllerpkg.Interface)
		for n, fn := range controllerpkg.Known() {
			logrus.Infof("Register controller: %s", n)
			controllers[n] = fn(ctx)
		}

		var wg sync.WaitGroup
		for n, cRoutine := range controllers {
			wg.Add(1)
			go func(n string, cRoutine controllerpkg.Interface) {
				defer wg.Done()

				logrus.Infof("Starting controller: %s", n)
				err := cRoutine(2, stopCh)

				if err != nil {
					logrus.Fatalf("error running %s controller: %s", n, err.Error())
				}
			}(n, cRoutine)
		}
		ctx.SharedInformerFactory.Start(stopCh)
		//ctx.KubeSharedInformerFactory.Start(stopCh)
		wg.Wait() // wait for controllers to finish
		logrus.Fatalf("Control loops exited")
	}

	startLeadingElection(run)

	panic("unreachable")
}

func newControllerContext() *controllerpkg.Context {
	kubecli := k8sutil.MustNewKubeClient()
	intcl := getClientSet()
	sIF := getSharedInformerFactory(intcl)

	return &controllerpkg.Context{
		Namespace:             opt.Namespace,
		KubeCli:               kubecli,
		KubeExtCli:            k8sutil.MustNewKubeExtClient(),
		SharedInformerFactory: sIF,
		MCClient:              intcl,
	}
}

func getClientSet() clientset.Interface {
	kubeCfg, err := k8sutil.ClusterConfig()
	if err != nil {
		logrus.Fatalf("fail to get clientset: %v", err)
	}

	intcl, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		logrus.Fatalf("fail to get clientset: %v", err)
	}
	return intcl
}

func getSharedInformerFactory(intcl clientset.Interface) informers.SharedInformerFactory {

	sharedInformerFactory := informers.NewFilteredSharedInformerFactory(intcl,
		opt.InformersResyncTime, opt.Namespace, nil)
	return sharedInformerFactory

}

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

func startLeadingElection(run func(<-chan struct{})) {
	kubecli := k8sutil.MustNewKubeClient()

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          getResourceLock(kubecli),
		LeaseDuration: opt.LeaderElectionLeaseDuration,
		RenewDeadline: opt.LeaderElectionRenewDeadline,
		RetryPeriod:   opt.LeaderElectionRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				logrus.Fatalf("Leader election lost")
			},
		},
	})
}

func getResourceLock(kubecli kubernetes.Interface) resourcelock.Interface {
	rl, err := resourcelock.New(resourcelock.EndpointsResourceLock,
		opt.Namespace,
		"mysql-operator-titanium",
		kubecli.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      util.GetPodHostName(),
			EventRecorder: util.CreateEventRecorder(kubecli, opt.PodName, opt.Namespace),
		})
	if err != nil {
		logrus.Fatalf("error creating lock: %v", err)
	}
	return rl
}
