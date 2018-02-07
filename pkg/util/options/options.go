package options

import (
	"strings"
	"sync"

	"github.com/spf13/pflag"
	"k8s.io/api/core/v1"
)

type Options struct {
	mysqlImage string

	MysqlImage    string
	MysqlImageTag string

	TitaniumImage string

	ImagePullSecretName string
	ImagePullPolicy     v1.PullPolicy
}

const (
	defaultMysqlImage    = "percona:5.7"
	defaultTitaniumImage = "gcr.io/pl-infra/titanium-operator:latest"

	defaultImagePullPolicy = v1.PullIfNotPresent
)

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.mysqlImage, "mysql-image", defaultMysqlImage,
		"The mysql image. Default to "+defaultMysqlImage)
	fs.StringVar(&o.TitaniumImage, "titanium-image", defaultTitaniumImage,
		"The image that instrumentate mysql. Default to "+defaultTitaniumImage)

	fs.StringVar(&o.ImagePullSecretName, "pull-secret", "",
		"The secret name for used as pull secret. Default none.")

}

var instance *Options
var once sync.Once

func GetOptions() *Options {
	once.Do(func() {
		instance = &Options{
			mysqlImage:    defaultMysqlImage,
			TitaniumImage: defaultTitaniumImage,

			ImagePullPolicy: defaultImagePullPolicy,
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
