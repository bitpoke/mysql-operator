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

	"github.com/bitpoke/mysql-operator/pkg/version"
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
	// SidecarMysql57Image is the image used in sidecar container to serve backups and configure MySQL
	SidecarMysql57Image string
	// SidecarMysql8Image as above but used when cluster uses mysql 8.0 and above
	SidecarMysql8Image string

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

	// OrchestratorConcurrentReconciles sets the orchestrator controller workers
	OrchestratorConcurrentReconciles int32

	// FailoverBeforeShutdownEnabled if enabled inserts a pre-stop lifecycle hook into pod
	// to trigger a failover before shutdown
	FailoverBeforeShutdownEnabled bool

	// AllowCrossNamespaceUser allow creating users resources in clusters that are not in the same namespace.
	AllowCrossNamespaceUsers bool

	// AllowCrossNamespaceDatabase allow creating users resources in clusters that are not in the same namespace.
	AllowCrossNamespaceDatabases bool
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
	defaultExporterImage = "prom/mysqld-exporter:v0.13.0"

	defaultImagePullPolicy     = corev1.PullIfNotPresent
	defaultImagePullSecretName = ""

	defaultOrchestratorTopologyUser     = ""
	defaultOrchestratorTopologyPassword = ""

	defaultLeaderElectionNamespace = "default"
	defaultLeaderElectionID        = "mysql-operator-leader-election"

	defaultNamespace = ""

	defaultFailoverBeforeShutdownEnabled = true
)

var (
	defaultSidecarMysql57Image = "docker.io/bitpoke/mysql-operator-sidecar-5.7:" + version.GetInfo().GitVersion
	defaultSidecarMysql8Image  = "docker.io/bitpoke/mysql-operator-sidecar-8.0:" + version.GetInfo().GitVersion
)

// AddFlags registers all mysql-operator needed flags
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.SidecarMysql57Image, "sidecar-image", defaultSidecarMysql57Image,
		"The image that is used for mysql node instrumentation.")

	fs.StringVar(&o.SidecarMysql8Image, "sidecar-mysql8-image", defaultSidecarMysql8Image,
		"The image that is used for mysql (version 8.0 or above) node instrumentation.")

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

	fs.Int32Var(&o.OrchestratorConcurrentReconciles, "orchestrator-concurrent-reconciles", 10,
		"Set the number of workers for orchestrator reconciler.")

	fs.BoolVar(&o.FailoverBeforeShutdownEnabled, "failover-before-shutdown", defaultFailoverBeforeShutdownEnabled,
		"In pre-stop hook trigger a failover from Orchestrator")

	fs.BoolVar(&o.AllowCrossNamespaceUsers, "allow-cross-namespace-user", false,
		"Allow the operator create users in clusters from other namespaces. Enabling this may be a security issue")

	fs.BoolVar(&o.AllowCrossNamespaceDatabases, "allow-cross-namespace-database", false,
		"Allow the operator create database in clusters from other namespaces. Enabling this may be a security issue")
}

var instance *Options
var once sync.Once

// GetOptions returns a singleton that contains all options
func GetOptions() *Options {
	once.Do(func() {
		instance = &Options{
			SidecarMysql57Image:  defaultSidecarMysql57Image,
			SidecarMysql8Image:   defaultSidecarMysql8Image,
			MetricsExporterImage: defaultExporterImage,

			ImagePullPolicy:     defaultImagePullPolicy,
			ImagePullSecretName: defaultImagePullSecretName,

			OrchestratorTopologyUser:     defaultOrchestratorTopologyUser,
			OrchestratorTopologyPassword: defaultOrchestratorTopologyPassword,

			Namespace: defaultNamespace,

			FailoverBeforeShutdownEnabled: defaultFailoverBeforeShutdownEnabled,
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
