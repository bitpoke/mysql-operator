package options

import (
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/api/core/v1"
)

type Options struct {
	mysqlImage string

	MysqlImage    string
	MysqlImageTag string

	TitaniumImage string

	MetricsExporterImage string

	ImagePullSecretName string
	ImagePullPolicy     v1.PullPolicy

	OrchestratorUri                string
	OrchestratorTopologySecretName string

	JobCompleteSuccessGraceTime time.Duration
}

const (
	defaultMysqlImage    = "percona:5.7"
	defaultTitaniumImage = "gcr.io/pl-infra/titanium-toolbox:latest"
	defaultExporterImage = "prom/mysqld-exporter:latest"

	defaultImagePullPolicy = v1.PullIfNotPresent
	orcURI                 = ""
	orcSCRT                = ""
)

var (
	defaultJobGraceTime = 24 * time.Hour
)

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.mysqlImage, "mysql-image", defaultMysqlImage,
		"The mysql image.")
	fs.StringVar(&o.TitaniumImage, "titanium-toolbox-image", defaultTitaniumImage,
		"The image that instrumentate mysql.")
	fs.StringVar(&o.MetricsExporterImage, "metrics-exporter-image", defaultExporterImage,
		"The image for mysql metrics exporter.")
	fs.StringVar(&o.ImagePullSecretName, "pull-secret", "",
		"The secret name for used as pull secret.")
	fs.StringVar(&o.OrchestratorUri, "orchestrator-uri", orcURI,
		"The orchestrator uri")
	fs.StringVar(&o.OrchestratorTopologySecretName, "orchestrator-secret", orcURI,
		"The orchestrator topology secret name.")
	fs.DurationVar(&o.JobCompleteSuccessGraceTime, "job-grace-time", defaultJobGraceTime,
		"The time in hours how jobs after completion are keept.")
}

var instance *Options
var once sync.Once

func GetOptions() *Options {
	once.Do(func() {
		instance = &Options{
			mysqlImage:           defaultMysqlImage,
			TitaniumImage:        defaultTitaniumImage,
			MetricsExporterImage: defaultExporterImage,

			ImagePullPolicy:             defaultImagePullPolicy,
			JobCompleteSuccessGraceTime: defaultJobGraceTime,
		}
	})

	return instance
}

func (o *Options) Validate() error {
	// Update mysql image and tag.
	i := strings.Split(o.mysqlImage, ":")
	o.MysqlImage = i[0]
	o.MysqlImageTag = i[1]
	return nil
}
