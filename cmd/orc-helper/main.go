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

package main

import (
	"log"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/presslabs/mysql-operator/pkg/apis"
	"github.com/presslabs/mysql-operator/pkg/orc-helper"
)

var (
	c client.Client
)

func main() {

	cmd := &cobra.Command{
		Use:   "orchestrator-helper",
		Short: "Helper for orchestrator.",
		Long:  `This command is a helper for updating MySQL cluster resources. Record events.`,
		Run: func(cmd *cobra.Command, args []string) {
			log.Fatal("you run orchestrator helper, see help section")
		},
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("unable to get configuration: ", err)
	}

	// Setup Scheme for all resources
	s := scheme.Scheme
	if err = apis.AddToScheme(s); err != nil {
		log.Fatal("unable to register types to scheme: ", err)
	}

	// initialize k8s client
	c, err = client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		log.Fatal("unable to get the k8s client: ", err)
	}

	fipCmd := &cobra.Command{
		Use:   "failover-in-progress",
		Short: "Set failover in progress condition for given cluster",
		Run: func(cmd *cobra.Command, args []string) {
			// check command line args
			if len(args) != 2 {
				log.Fatal("see usage: <cluster.name> <message>")
			}

			err = orchelper.UpdateClusterFailoverCond(c, args[0], "OrcFailoverInProgress", args[1], true)
			if err != nil {
				log.Fatal("error in updating cluster: ", err)
			}
		},
	}
	cmd.AddCommand(fipCmd)

	evWarningType := false
	evCmd := &cobra.Command{
		Use:   "event",
		Short: "Set event on a given cluster",
		Run: func(cmd *cobra.Command, args []string) {
			// check command line args
			if len(args) != 3 {
				log.Fatal("see usage: <cluster.name> <event-name> <message> [-warning]")
			}

			err = orchelper.UpdateEventForCluster(c, s, args[0], args[1], args[2], evWarningType)
			if err != nil {
				log.Fatal("error in updating cluster: ", err)
			}
		},
	}
	evCmd.Flags().BoolVarP(&evWarningType, "warning", "w", false, "if it's a warning event in k8s")
	cmd.AddCommand(evCmd)

	if err := cmd.Execute(); err != nil {
		log.Fatal("failed to execute command: ", err)
	}
}
