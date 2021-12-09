module github.com/bitpoke/mysql-operator

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-ini/ini v1.62.0
	github.com/go-logr/logr v0.4.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/go-test/deep v1.0.7
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/imdario/mergo v0.3.12
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/presslabs/controller-util v0.3.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	gopkg.in/ini.v1 v1.57.0 // indirect

	// kubernetes
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/controller-runtime v0.9.7
	sigs.k8s.io/testing_frameworks v0.1.2
)
