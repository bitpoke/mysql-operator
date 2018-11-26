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

	customLog "github.com/presslabs/mysql-operator/pkg/util/log"
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/presslabs/mysql-operator/pkg/apis"
	"github.com/presslabs/mysql-operator/pkg/controller"
	"github.com/presslabs/mysql-operator/pkg/options"
	"github.com/presslabs/mysql-operator/pkg/util/stop"
)

var log = logf.Log.WithName("mysql-operator")

func main() {
	fs := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	fs.AddGoFlagSet(flag.CommandLine)

	opt := options.GetOptions()
	opt.AddFlags(fs)
	if err := fs.Parse(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse command line args, see help, err: %s", err)
		os.Exit(1)
	}

	// for glog
	if err := flag.Lookup("logtostderr").Value.Set("true"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set glog to use stderr, err: %s", err)
	}

	// set logging
	logf.SetLogger(customLog.ZapLogger())

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
	if err := mgr.Start(stop.Channel); err != nil {
		log.Error(err, "unable to start the manager")
		os.Exit(1)
	}
}
