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

package mysqlcluster

import (
	"fmt"
)

// Validate checks if the cluster spec is validated
func (c *MysqlCluster) Validate() error {
	// TODO: this validation should be done in an admission web-hook
	if len(c.Spec.SecretName) == 0 {
		return fmt.Errorf("spec.secretName is missing")
	}

	if len(c.GetMysqlImage()) == 0 {
		return fmt.Errorf("%s is not a valid MySQL version", c.Spec.MysqlVersion)
	}

	// volume spec should be specified on the cluster
	vs := c.Spec.VolumeSpec
	if anyIsNull(vs.PersistentVolumeClaim, vs.HostPath, vs.EmptyDir) {
		return fmt.Errorf("no .spec.volumeSpec is specified")
	}

	return nil
}

// anyNull checks if any of the given parameters is null and returns true if so
func anyIsNull(vars ...interface{}) bool {
	isNull := false
	for _, v := range vars {
		isNull = isNull || v == nil
	}
	return isNull
}
