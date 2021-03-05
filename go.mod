module github.com/presslabs/mysql-operator

go 1.13

require (
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/go-etcd v2.0.0+incompatible // indirect
	github.com/cpuguy83/go-md2man v1.0.10 // indirect
	github.com/go-ini/ini v1.57.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/go-test/deep v1.0.7
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/imdario/mergo v0.3.11
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/presslabs/controller-util v0.3.0-alpha.2
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/ugorji/go/codec v0.0.0-20181204163529-d75b2dcb6bc8 // indirect
	github.com/wgliang/cron v0.0.0-20180129105837-79834306f643
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77 // indirect
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb
	gopkg.in/ini.v1 v1.57.0 // indirect

	// kubernetes
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.6.0
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/controller-tools v0.5.0 // indirect
	sigs.k8s.io/testing_frameworks v0.1.2
)
