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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
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
	"github.com/presslabs/mysql-operator/pkg/util"
	"github.com/presslabs/mysql-operator/pkg/util/kube"
	goptions "github.com/presslabs/mysql-operator/pkg/util/options"

	"github.com/presslabs/mysql-operator/pkg/controller"
	// Register all available controllers
	_ "github.com/presslabs/mysql-operator/pkg/controller/backupscontroller"
	_ "github.com/presslabs/mysql-operator/pkg/controller/clustercontroller"
)

const controllerAgentName = "titanium-controller"

// NewTitaniumControllerCommand creates a new cobra command for running the titanium controller
func NewTitaniumControllerCommand(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := options.NewTitaniumControllerOptions()
	gOpt := goptions.GetOptions()

	cmd := &cobra.Command{
		Use:   "titanium-controller",
		Short: fmt.Sprintf("Managed WordPress deployments for kubernetes (%s) (%s)", util.AppVersion, util.AppGitCommit),
		Long: `
titanium-operator is a Kubernetes addon to automate the deployment, scaling and
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
			RunTitaniumController(o, stopCh)
		},
	}
	flags := cmd.Flags()
	o.AddFlags(flags)
	gOpt.AddFlags(flags)

	return cmd
}

// RunTitaniumController starts the titanium reconcile loops
func RunTitaniumController(opts *options.TitaniumControllerOptions, stopCh <-chan struct{}) {
	glog.Info("Start controller...")
	ctx, kubeCfg, err := buildControllerContext(opts)
	if err != nil {
		glog.Fatalf(err.Error())
	}

	// start probing http server
	httpServer(stopCh, opts.ProbeAddr)

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
		glog.V(4).Infof("Starting shared informer factory")
		ctx.SharedInformerFactory.Start(stopCh)
		ctx.KubeSharedInformerFactory.Start(stopCh)
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

func buildControllerContext(opts *options.TitaniumControllerOptions) (*controller.Context, *rest.Config, error) {
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

	// Create event broadcaster
	// Add titanium types to the default Kubernetes Scheme so Events can be
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
	}, kubeCfg, nil
}

func startLeaderElection(opts *options.TitaniumControllerOptions, leaderElectionClient kubernetes.Interface, recorder record.EventRecorder, run func(<-chan struct{})) {
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

func httpServer(stop <-chan struct{}, addr string) {
	mux := http.NewServeMux()

	// Add health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Shutdown gracefully the http server
	go func() {
		<-stop // wait for stop signal
		if err := srv.Shutdown(context.Background()); err != nil {
			glog.Errorf("Failed to stop probe server, err: %s", err)
		}
	}()

	go func() {
		glog.Fatal(srv.ListenAndServe())
	}()
}
