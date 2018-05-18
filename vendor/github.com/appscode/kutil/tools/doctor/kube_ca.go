package doctor

import (
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
)

func (d *Doctor) extractKubeCA(info *ClusterInfo) error {
	info.ClientConfig.Host = d.config.Host
	info.ClientConfig.Insecure = d.config.Insecure

	if len(d.config.CAData) > 0 {
		info.ClientConfig.CAData = strings.TrimSpace(string(d.config.CAData))
	} else if len(d.config.CAFile) > 0 {
		data, err := ioutil.ReadFile(d.config.CAFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load ca file %s", d.config.CAFile)
		}
		info.ClientConfig.CAData = strings.TrimSpace(string(data))
	}
	return nil
}
