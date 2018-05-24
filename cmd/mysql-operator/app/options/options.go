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
	"fmt"
	"time"

	"github.com/spf13/pflag"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type MysqlControllerOptions struct {
	APIServerHost            string
	Namespace                string
	ClusterResourceNamespace string

	LeaderElect                 bool
	LeaderElectionNamespace     string
	LeaderElectionLeaseDuration time.Duration
	LeaderElectionRenewDeadline time.Duration
	LeaderElectionRetryPeriod   time.Duration

	// The number of workers
	NoWorkers int

	InstallCRDs bool
}

const (
	defaultAPIServerHost = ""
	defaultNamespace     = "default"

	defaultLeaderElect                 = true
	defaultLeaderElectionNamespace     = "kube-system"
	defaultLeaderElectionLeaseDuration = 15 * time.Second
	defaultLeaderElectionRenewDeadline = 10 * time.Second
	defaultLeaderElectionRetryPeriod   = 2 * time.Second

	defaultNoWorkers   = 5
	defaultInstallCRDs = false
)

func NewControllerOptions() *MysqlControllerOptions {
	return &MysqlControllerOptions{
		APIServerHost:               defaultAPIServerHost,
		Namespace:                   defaultNamespace,
		LeaderElect:                 defaultLeaderElect,
		LeaderElectionNamespace:     defaultLeaderElectionNamespace,
		LeaderElectionLeaseDuration: defaultLeaderElectionLeaseDuration,
		LeaderElectionRenewDeadline: defaultLeaderElectionRenewDeadline,
		LeaderElectionRetryPeriod:   defaultLeaderElectionRetryPeriod,

		NoWorkers: defaultNoWorkers,
	}
}

func (s *MysqlControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.APIServerHost, "master", defaultAPIServerHost, ""+
		"Optional apiserver host address to connect to. If not specified, autoconfiguration "+
		"will be attempted.")
	fs.StringVar(&s.Namespace, "namespace", defaultNamespace, ""+
		"Namespace to monitor resources within. If not specified, default "+
		"namespace will be watched")

	fs.BoolVar(&s.LeaderElect, "leader-elect", true, ""+
		"If true, mysql-controller will perform leader election between instances "+
		"to ensure no more than one instance of mysql-controller operates at a time")
	fs.StringVar(&s.LeaderElectionNamespace, "leader-election-namespace",
		defaultLeaderElectionNamespace, ""+
			"Namespace used to perform leader election. Only used if leader election is enabled")
	fs.DurationVar(&s.LeaderElectionLeaseDuration, "leader-election-lease-duration",
		defaultLeaderElectionLeaseDuration, ""+
			"The duration that non-leader candidates will wait after observing a leadership "+
			"renewal until attempting to acquire leadership of a led but unrenewed leader "+
			"slot. This is effectively the maximum duration that a leader can be stopped "+
			"before it is replaced by another candidate. This is only applicable if leader "+
			"election is enabled.")
	fs.DurationVar(&s.LeaderElectionRenewDeadline, "leader-election-renew-deadline",
		defaultLeaderElectionRenewDeadline, ""+
			"The interval between attempts by the acting master to renew a leadership slot "+
			"before it stops leading. This must be less than or equal to the lease duration. "+
			"This is only applicable if leader election is enabled.")
	fs.DurationVar(&s.LeaderElectionRetryPeriod, "leader-election-retry-period",
		defaultLeaderElectionRetryPeriod, ""+
			"The duration the clients should wait between attempting acquisition and renewal "+
			"of a leadership. This is only applicable if leader election is enabled.")

	fs.IntVar(&s.NoWorkers, "workers", defaultNoWorkers, "The number of workers that"+
		" process events.")

	fs.BoolVar(&s.InstallCRDs, "install-crds", defaultInstallCRDs, "Whether or not to install CRDs.")

}

func (o *MysqlControllerOptions) Validate() error {
	var errs []error
	if len(o.Namespace) == 0 {
		errs = append(errs, fmt.Errorf("no namespace specified"))
	}
	return utilerrors.NewAggregate(errs)
}
