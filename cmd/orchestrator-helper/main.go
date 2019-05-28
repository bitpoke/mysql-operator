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
	"os"

	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/presslabs/mysql-operator/pkg/apis"
	"github.com/presslabs/mysql-operator/pkg/orc-helper"
)

var (
	client kclient.Client
)

func main() {
	// check command line args
	if len(os.Args) != 2 {
		log.Fatal("see usage: <cluster.name>")
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("unable to get configuration", err)
	}

	// Setup Scheme for all resources
	s := scheme.Scheme
	if err = apis.AddToScheme(s); err != nil {
		log.Fatal("unable to register types to scheme", err)
	}

	// initialize k8s client
	client, err = kclient.New(cfg, kclient.Options{Scheme: s})
	if err != nil {
		log.Fatal("unable to get the k8s client", err)
	}

	err = orchelper.UpdateClusterFailoverCond(client, os.Args[1], "orcFailoverInProgress", "Orc failover in progress", true)
	if err != nil {
		log.Fatal("error in updating cluster: ", err)
	}
}
