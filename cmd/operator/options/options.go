package options

import (
	"flag"
	"time"
)

type ControllerOptions struct {
	APIServerHost            string
	ClusterResourceNamespace string

	Namespace string
	PodName   string

	LeaderElect                 bool
	LeaderElectionNamespace     string
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration

	InformersResyncTime time.Duration
}

const (
	defaultClusterResourceNamespace = "kube-system"

	defaultLeaderElect                 = true
	defaultLeaderElectionNamespace     = "kube-system"
	defaultLeaderElectionLeaseDuration = 15 * time.Second
	defaultLeaderElectionRenewDeadline = 10 * time.Second
	defaultLeaderElectionRetryPeriod   = 2 * time.Second
)

func NewControllerOptions() *ControllerOptions {
	return &ControllerOptions{
		Namespace:                   "default",
		PodName:                     "pod-name",
		ClusterResourceNamespace:    defaultClusterResourceNamespace,
		LeaderElect:                 defaultLeaderElect,
		LeaderElectionNamespace:     defaultLeaderElectionNamespace,
		LeaderElectionLeaseDuration: defaultLeaderElectionLeaseDuration,
		LeaderElectionRenewDeadline: defaultLeaderElectionRenewDeadline,
		LeaderElectionRetryPeriod:   defaultLeaderElectionRetryPeriod,
		InformersResyncTime:         30 * time.Second,
	}
}

func (s *ControllerOptions) AddFlags() {
	flag.StringVar(&s.Namespace, "namespace", "default", ""+
		"Optional namespace to monitor resources within. This can be used to limit the scope "+
		"of cert-manager to a single namespace. If not specified, all namespaces will be watched")

	flag.StringVar(&s.PodName, "pod-name", "pod-name", ""+
		"Optional pod name, when running out of cluster.")

	flag.StringVar(&s.ClusterResourceNamespace, "resource-namespace",
		defaultClusterResourceNamespace,
		"Namespace to store resources owned by cluster scoped resources.")

	flag.BoolVar(&s.LeaderElect, "leader-elect", true, ""+
		"If true, cert-manager will perform leader election between instances to ensure no more "+
		"than one instance of cert-manager operates at a time")

}

// here can be done some custom validation if needed.
func (o *ControllerOptions) Validate() error {
	return nil
}
