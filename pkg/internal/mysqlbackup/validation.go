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

package mysqlbackup

import (
	"fmt"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

// Validate checks if the backup spec is validated
func (c *MysqlBackup) Validate(cluster *mysqlcluster.MysqlCluster) error {
	// TODO: this validation should be done in an admission web-hook

	if c.Spec.CandidateNode != "" {
		validate := false
		for i := 0; i < int(*cluster.Spec.Replicas); i++ {
			if c.Spec.CandidateNode == cluster.GetPodHostname(i) {
				validate = true
				break
			}
		}
		if !validate {
			return fmt.Errorf("spec.candidateNode is not a valid node")
		}
	}
	return nil
}
