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

package options

import (
	"os"
	"sync"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"

	"github.com/presslabs/mysql-operator/pkg/util"
)

// nolint: unparam
func getFromEnvOrDefault(key, def string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return def
	}
	return value
}

// Options is the data structure that contains information about mysql operator configuration
type Options struct {
	// SidecarImage is the image used in sidecar container to serve backups and configure MySQL
	SidecarImage string

	// MetricsExporterImage is the image for exporter container
	MetricsExporterImage string

	// ImagePullSecretName is the secret name where are found secrets for pulling images. This is
	// the default value and may be overwrite by the cluster .spec.podSpec.imagePullSecrets field.
	ImagePullSecretName string
	// ImagePullPolicy is the default image pull policy
	ImagePullPolicy corev1.PullPolicy

	// OrchestratorURI represents the URI where the Orchestrator can be reached.
	OrchestratorURI string
	// OrchestratorTopologyPassword is the password that is used by Orchestrator to connect to MySQL
	// nodes. This field is set in cluster secret as well.
	OrchestratorTopologyPassword string
	// OrchestratorTopologyUser is the user that is used by Orchestrator to connect to MySQL
	// nodes. This field is set in cluster secret as well.
	OrchestratorTopologyUser string

	// LeaderElectionNamespace the namespace where the lock resource will be created
	LeaderElectionNamespace string
	// LederElectionID the name of the lock resource
	LeaderElectionID string

	// Namespace where to look after objects. This will limit the operator action range.
	Namespace string

	// MySQLVersionImageOverride define a map between MySQL version and image.
	// This overrides the default versions and has priority.
	MySQLVersionImageOverride map[string]string
}

type pullpolicy corev1.PullPolicy

func (pp *pullpolicy) String() string {
	return string(*pp)
}

func (pp *pullpolicy) Set(value string) error {
	*pp = pullpolicy(value)
	return nil
}

func (pp *pullpolicy) Type() string {
	return "v1.PullPolicy"
}

// nolint: unparam
func newPullPolicyValue(defaultValue corev1.PullPolicy, v *corev1.PullPolicy) *pullpolicy {
	*v = defaultValue
	return (*pullpolicy)(v)
}

const (
	defaultExporterImage = "prom/mysqld-exporter:v0.11.0"

	defaultImagePullPolicy     = corev1.PullIfNotPresent
	defaultImagePullSecretName = ""

	defaultOrchestratorTopologyUser     = ""
	defaultOrchestratorTopologyPassword = ""

	defaultLeaderElectionNamespace = "default"
	defaultLeaderElectionID        = "mysql-operator-leader-election"

	defaultNamespace = ""
)

var (
	defaultSidecarImage = "quay.io/presslabs/mysql-operator-sidecar:" + util.AppVersion
)

// AddFlags registers all mysql-operator needed flags
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.SidecarImage, "sidecar-image", defaultSidecarImage,
		"The image that instrumentate mysql.")

	fs.StringVar(&o.MetricsExporterImage, "metrics-exporter-image", defaultExporterImage,
		"The image for mysql metrics exporter.")
	fs.StringVar(&o.ImagePullSecretName, "image-pull-secret", defaultImagePullSecretName,
		"The secret name for used as pull secret.")

	fs.VarP(newPullPolicyValue(defaultImagePullPolicy, &o.ImagePullPolicy),
		"image-pull-policy", "", "Set image pull policy.")

	fs.StringVar(&o.OrchestratorURI, "orchestrator-uri", "",
		"The orchestrator uri")
	fs.StringVar(&o.OrchestratorTopologyPassword, "orchestrator-topology-password", defaultOrchestratorTopologyUser,
		"The orchestrator topology password. Can also be set as ORC_TOPOLOGY_PASSWORD environment variable.")
	fs.StringVar(&o.OrchestratorTopologyUser, "orchestrator-topology-user", defaultOrchestratorTopologyPassword,
		"The orchestrator topology user. Can also be set as ORC_TOPOLOGY_USER environment variable.")

	fs.StringVar(&o.LeaderElectionNamespace, "leader-election-namespace", defaultLeaderElectionNamespace,
		"The leader election namespace.")
	fs.StringVar(&o.LeaderElectionID, "leader-election-id", defaultLeaderElectionID,
		"The leader election id.")

	fs.StringVar(&o.Namespace, "namespace", defaultNamespace,
		"The namespace to restrict the client to watch objects.")

	fs.StringToStringVar(&o.MySQLVersionImageOverride, "mysql-versions-to-image", map[string]string{},
		"A map to override default image for different mysql versions. Example: 5.7.23=mysql:5.7,5.7.24=mysql:5.7")
}

var instance *Options
var once sync.Once

// GetOptions returns a singlethon that contains all options
func GetOptions() *Options {
	once.Do(func() {
		instance = &Options{
			SidecarImage:         defaultSidecarImage,
			MetricsExporterImage: defaultExporterImage,

			ImagePullPolicy:     defaultImagePullPolicy,
			ImagePullSecretName: defaultImagePullSecretName,

			OrchestratorTopologyUser:     defaultOrchestratorTopologyUser,
			OrchestratorTopologyPassword: defaultOrchestratorTopologyPassword,

			Namespace: defaultNamespace,
		}
	})

	return instance
}

// Validate validate the command line values
func (o *Options) Validate() error {

	if len(o.OrchestratorTopologyUser) == 0 {
		o.OrchestratorTopologyUser = getFromEnvOrDefault("ORC_TOPOLOGY_USER", "")
	}
	if len(o.OrchestratorTopologyPassword) == 0 {
		o.OrchestratorTopologyPassword = getFromEnvOrDefault("ORC_TOPOLOGY_PASSWORD", "")
	}
	return nil
}
