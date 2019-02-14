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

package main

import (
	"flag"
	"fmt"
	"os"

	logf "github.com/presslabs/controller-util/log"
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/presslabs/mysql-operator/pkg/apis"
	"github.com/presslabs/mysql-operator/pkg/controller"
	"github.com/presslabs/mysql-operator/pkg/options"
)

var log = logf.Log.WithName("mysql-operator")

func main() {
	fs := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	fs.AddGoFlagSet(flag.CommandLine)

	debug := false
	fs.BoolVar(&debug, "debug", false, "Set logger in debug mode")

	opt := options.GetOptions()
	opt.AddFlags(fs)
	if err := fs.Parse(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse command line args, see help, err: %s", err)
		os.Exit(1)
	}

	// set logging
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	klogFlags.Set("logtostderr", "true")      // nolint: errcheck
	klogFlags.Set("alsologtostderr", "false") // nolint: errcheck

	logf.SetLogger(logf.ZapLogger(debug))

	if err := opt.Validate(); err != nil {
		log.Error(err, "failed to validate command line args, see help.")
		os.Exit(1)
	}

	log.Info("Starting mysql-operator...")

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get configuration")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		LeaderElection:          true,
		LeaderElectionNamespace: opt.LeaderElectionNamespace,
		LeaderElectionID:        opt.LeaderElectionID,
		Namespace:               opt.Namespace,
	})
	if err != nil {
		log.Error(err, "unable to create a new manager")
		os.Exit(1)
	}

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to register types to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "unable to setup controllers")
		os.Exit(1)
	}

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "unable to start the manager")
		os.Exit(1)
	}
}
