/*
Copyright 2018 Platform9 Inc

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
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/presslabs/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/presslabs/mysql-operator/pkg/options"
)

const (
	deleteSuccess        = "SucessfulDelete"
	deleteFail           = "FailedDelete"
	messagePvcDeleted    = "delete Claim %s in StatefulSet %s successful"
	messagePvcNotDeleted = "delete Claim %s in StatefulSet %s failed"
)

var log = logf.Log.WithName("mysqlcluster.pvccleaner")

// PvcCleaner represents an object to clean Pvcs of a MysqlCluster
type PvcCleaner struct {
	cluster *mysqlcluster.MysqlCluster
	opt     *options.Options
}

// NewPvcCleaner returns a new PVC cleaner object
func NewPvcCleaner(cluster *mysqlcluster.MysqlCluster, opt *options.Options) *PvcCleaner {

	return &PvcCleaner{
		cluster: cluster,
		opt:     opt,
	}
}

// Run performs cleanup of orphaned pvcs in a Mysql cluster
func (p *PvcCleaner) Run(ctx context.Context, c client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) error {
	meta := p.cluster.ObjectMeta
	key := types.NamespacedName{Name: meta.GetName(), Namespace: meta.GetNamespace()}

	cluster := api.MysqlCluster{}
	err := c.Get(ctx, key, &cluster)
	if err != nil {
		return err
	}

	if cluster.DeletionTimestamp != nil {
		log.V(4).Info("being deleted, no action", "MysqlCluster", key)
		return nil
	}

	replicas := cluster.Spec.Replicas

	// Find any pvcs with higher ordinal than replicas and delete them
	claims, err := p.getClaims(ctx, c)
	if err != nil {
		return err
	}

	keys := getKeys(claims)
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	for _, k := range keys {
		if int32(k) >= replicas {
			log.Info("cleaning up", "pvc", claims[k])
			if err := deleteClaim(ctx, c, recorder, &cluster, claims[k]); err != nil {
				log.Error(err, "deleting claim")
				return err
			}
		}
	}
	return nil
}

func deleteClaim(ctx context.Context, c client.Client, recorder record.EventRecorder,
	cluster *api.MysqlCluster, pvc *core.PersistentVolumeClaim) error {
	err := c.Delete(ctx, pvc)
	if err != nil {
		recorder.Event(cluster, core.EventTypeWarning, deleteFail,
			fmt.Sprintf(messagePvcNotDeleted, pvc.Name, cluster.Name))
		return err
	}

	recorder.Event(cluster, core.EventTypeNormal, deleteSuccess,
		fmt.Sprintf(messagePvcDeleted, pvc.Name, cluster.Name))
	return nil
}

func (p *PvcCleaner) getClaims(ctx context.Context, c client.Client) (map[int]*core.PersistentVolumeClaim, error) {
	meta := p.cluster.ObjectMeta
	pvcs := &core.PersistentVolumeClaimList{}
	lo := &client.ListOptions{
		Namespace:     meta.GetNamespace(),
		LabelSelector: labels.SelectorFromSet(p.cluster.GetLabels()),
	}
	err := c.List(ctx, lo, pvcs)

	if err != nil {
		return nil, err
	}

	return byOrdinal(pvcs), nil
}

func byOrdinal(allClaims *core.PersistentVolumeClaimList) map[int]*core.PersistentVolumeClaim {
	claims := map[int]*core.PersistentVolumeClaim{}

	for i := 0; i < len(allClaims.Items); i++ {
		if allClaims.Items[i].DeletionTimestamp != nil {
			log.V(2).Info("being deleted, skipping", "pvc", allClaims.Items[i].Name)
			continue
		}

		_, ordinal, err := extract(allClaims.Items[i].Name)
		if err != nil {
			continue
		}

		claims[ordinal] = &allClaims.Items[i]

	}
	return claims
}

func extract(pvcName string) (string, int, error) {
	idx := strings.LastIndexAny(pvcName, "-")
	if idx == -1 {
		return "", 0, fmt.Errorf("PVC does not belong to a StatefulSet")
	}

	name := pvcName[:idx]
	ordinal, err := strconv.Atoi(pvcName[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("PVC does not belong to a StatefulSet")
	}
	return name, ordinal, nil
}

func getKeys(m map[int]*core.PersistentVolumeClaim) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
