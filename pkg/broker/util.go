/*
Copyright 2019 Pressinfra SRL

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

package broker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func getNamespaceFromContext(rawCtx json.RawMessage) string {
	c := struct {
		Namespace string `json:"namespace,omitempty"`
	}{}

	err := json.Unmarshal(rawCtx, &c)
	if err != nil {
		log.Info("can't decode k8s context", "error", err, "raw_context", rawCtx)
		return DefaultNamespace
	}
	if len(c.Namespace) == 0 {
		return DefaultNamespace
	}

	return c.Namespace
}

func (sb *serviceBroker) getClusterForID(ctx context.Context, instanceID string) (*mysqlcluster.MysqlCluster, error) {
	cList := &api.MysqlClusterList{}
	optList := client.MatchingLabels(map[string]string{
		instanceIDLabel: instanceID,
	})
	err := sb.Client.List(ctx, optList, cList)
	if err != nil {
		return nil, err
	}

	if len(cList.Items) != 1 {
		return nil, fmt.Errorf("no cluster found or id conflict")
	}

	return mysqlcluster.New(&cList.Items[0]), nil
}

func (sb *serviceBroker) getUtilityUserPassword(ctx context.Context, cluster *mysqlcluster.MysqlCluster) (string, string, error) {
	sct := &corev1.Secret{}
	err := sb.Client.Get(ctx, types.NamespacedName{
		Name:      cluster.Spec.SecretName,
		Namespace: cluster.Namespace,
	}, sct)

	if err != nil {
		return "", "", err
	}

	var (
		ok   bool
		pass []byte
	)
	if pass, ok = sct.Data["ROOT_PASSWORD"]; !ok {
		return "", "", fmt.Errorf("no field ROOT_PASSWORD found in secret")
	}

	return "root", string(pass), nil
}

func (sb *serviceBroker) getMySQLClusterConn(ctx context.Context, instanceID string) (*mysqlcluster.MysqlCluster, connection, error) {
	// get the MySQL cluster
	cluster, err := sb.getClusterForID(ctx, instanceID)
	if err != nil {
		return nil, nil, err
	}

	// establish a mysql connection
	user, pass, err := sb.getUtilityUserPassword(ctx, cluster)
	conn := newConnection(cluster, user, pass)

	return cluster, conn, nil
}

func getBindUserAnnotation(bindID string) string {
	return fmt.Sprintf("%s/%s", bindingUserAnnotationPath, bindID)
}
