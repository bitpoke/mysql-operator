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

package app

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
	apiext_clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	clientset "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned"
	intscheme "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/scheme"
	informers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions"

	"github.com/presslabs/mysql-operator/cmd/mysql-operator/app/options"
	"github.com/presslabs/mysql-operator/pkg/http"
	"github.com/presslabs/mysql-operator/pkg/util"
	"github.com/presslabs/mysql-operator/pkg/util/kube"
	goptions "github.com/presslabs/mysql-operator/pkg/util/options"

	"github.com/presslabs/mysql-operator/pkg/controller"
	// Register all available controllers
	_ "github.com/presslabs/mysql-operator/pkg/controller/backupscontroller"
	_ "github.com/presslabs/mysql-operator/pkg/controller/clustercontroller"
)

const controllerAgentName = "mysql-controller"

// NewControllerCommand creates a new cobra command for running the mysql controller
func NewControllerCommand(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := options.NewControllerOptions()
	gOpt := goptions.GetOptions()

	cmd := &cobra.Command{
		Use:   "mysql-controller",
		Short: fmt.Sprintf("Managed WordPress deployments for kubernetes (%s) (%s)", util.AppVersion, util.AppGitCommit),
		Long: `
mysql-operator is a Kubernetes addon to automate the deployment, scaling and
management of multiple mysql clusters.`,
		// TODO: Refactor this function from this package
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Validate(); err != nil {
				glog.Fatalf("error validating options: %s", err.Error())
			}
			if err := gOpt.Validate(); err != nil {
				glog.Fatalf("error validating mysql controller options: %s", err.Error())
			}
			// go StartPrometheusMetricsServer(stopCh)
			RunController(o, stopCh)
		},
	}
	flags := cmd.Flags()
	o.AddFlags(flags)
	gOpt.AddFlags(flags)

	return cmd
}

// RunController starts the mysql reconcile loops
func RunController(opts *options.MysqlControllerOptions, stopCh <-chan struct{}) {
	glog.Info("Start controller...")
	ctx, kubeCfg, err := buildControllerContext(opts)
	if err != nil {
		glog.Fatalf(err.Error())
	}

	http.StartHttpServer()

	run := func(_ <-chan struct{}) {
		var wg sync.WaitGroup
		var controllers = make(map[string]controller.Interface)
		for n, fn := range controller.Known() {
			controllers[n] = fn(ctx)
		}
		for n, fn := range controllers {
			wg.Add(1)
			go func(n string, fn controller.Interface) {
				defer wg.Done()
				glog.V(2).Infof("Starting %s controller", n)

				err := fn(opts.NoWorkers, stopCh)

				if err != nil {
					glog.Fatalf("error running %s controller: %s", n, err.Error())
				}
			}(n, fn)
		}

		wg.Wait()
		glog.Fatalf("Control loops exited")
	}

	if !opts.LeaderElect {
		run(stopCh)
		return
	}

	leaderElectionClient, err := kubernetes.NewForConfig(rest.AddUserAgent(kubeCfg, "leader-election"))

	if err != nil {
		glog.Fatalf("error creating leader election client: %s", err.Error())
	}

	startLeaderElection(opts, leaderElectionClient, ctx.Recorder, run)
	panic("unreachable")
}

func buildControllerContext(opts *options.MysqlControllerOptions) (*controller.Context, *rest.Config, error) {
	// Load the users Kubernetes config
	kubeCfg, err := kube.KubeConfig(opts.APIServerHost)

	if err != nil {
		return nil, nil, fmt.Errorf("error creating rest config: %s", err.Error())
	}

	// Create a Navigator api client
	intcl, err := clientset.NewForConfig(kubeCfg)

	if err != nil {
		return nil, nil, fmt.Errorf("error creating internal group client: %s", err.Error())
	}

	// Create a Kubernetes api client
	cl, err := kubernetes.NewForConfig(kubeCfg)

	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubernetes client: %s", err.Error())
	}

	crdcl, err := apiext_clientset.NewForConfig(kubeCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubernetes CRD client: %s", err.Error())
	}

	// Create event broadcaster
	// Add mysql types to the default Kubernetes Scheme so Events can be
	// logged properly
	intscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.V(4).Infof)
	eventBroadcaster.StartRecordingToSink(
		&corev1.EventSinkImpl{Interface: cl.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme, v1.EventSource{Component: controllerAgentName})

	sharedInformerFactory := informers.NewFilteredSharedInformerFactory(
		intcl, time.Second*30, opts.Namespace, nil)
	kubeSharedInformerFactory := kubeinformers.NewFilteredSharedInformerFactory(
		cl, time.Second*30, opts.Namespace, nil)
	return &controller.Context{
		Client:                    intcl,
		KubeClient:                cl,
		Recorder:                  recorder,
		KubeSharedInformerFactory: kubeSharedInformerFactory,
		SharedInformerFactory:     sharedInformerFactory,
		Namespace:                 opts.Namespace,
		InstallCRDs:               opts.InstallCRDs,
		CRDClient:                 crdcl,
	}, kubeCfg, nil
}

func startLeaderElection(opts *options.MysqlControllerOptions, leaderElectionClient kubernetes.Interface, recorder record.EventRecorder, run func(<-chan struct{})) {
	// Identity used to distinguish between multiple controller manager instances
	id, err := os.Hostname()
	if err != nil {
		glog.Fatalf("error getting hostname: %s", err.Error())
	}

	// Lock required for leader election
	rl := resourcelock.EndpointsLock{
		EndpointsMeta: metav1.ObjectMeta{
			Namespace: opts.LeaderElectionNamespace,
			Name:      controllerAgentName,
		},
		Client: leaderElectionClient.CoreV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      id + "-external-" + controllerAgentName,
			EventRecorder: recorder,
		},
	}

	// Try and become the leader and start controller manager loops
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          &rl,
		LeaseDuration: opts.LeaderElectionLeaseDuration,
		RenewDeadline: opts.LeaderElectionRenewDeadline,
		RetryPeriod:   opts.LeaderElectionRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	})
}
