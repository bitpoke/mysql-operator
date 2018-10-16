/*
Copyright 2018 Platform9, Inc

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

package clustercontroller

import (
	"context"
	"fmt"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// CleanupRetryError Indicates retry on error
type CleanupRetryError struct {
	msg string
}

func (e *CleanupRetryError) Error() string {
	return e.msg
}

// Cleanup deletes any orphaned objects created by Mysql cluster
func (c *Controller) Cleanup(ctx context.Context, name string, namespace string) error {

	glog.Infof("Cleaning up cluster: %s", name)

	client := c.k8client

	lbs := map[string]string{
		"app":           "mysql-operator",
		"mysql_cluster": name}

	selector := labels.SelectorFromSet(lbs)
	listOpt := metav1.ListOptions{
		LabelSelector: selector.String()}

	podList, err := client.CoreV1().Pods(namespace).List(listOpt)

	if err != nil {
		return fmt.Errorf("listing pods: %s", err)
	}

	if len(podList.Items) > 0 {
		msg := fmt.Sprintf("Pods still terminating for cluster: %s", name)
		glog.V(2).Infof(msg)
		return &CleanupRetryError{msg: msg}
	}

	pvcList, err := client.CoreV1().PersistentVolumeClaims(namespace).List(listOpt)

	if err != nil {
		return fmt.Errorf("listing pvcs: %s", err)
	}

	for _, pvc := range pvcList.Items {
		err := client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(pvc.Name, nil)
		if err != nil {
			glog.Warningf("Error deleting pvc: %s", err)
		}
	}

	return nil
}
