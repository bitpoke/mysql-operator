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

	"github.com/presslabs/mysql-operator/pkg/options"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	wrapcluster "github.com/presslabs/mysql-operator/pkg/controller/internal/mysqlcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
)

const (
	controllerName       = "controller.mysqlcluster"
	deleteSuccess        = "SucessfulDelete"
	deleteFail           = "FailedDelete"
	messagePvcDeleted    = "delete Claim %s in StatefulSet %s successful"
	messagePvcNotDeleted = "delete Claim %s in StatefulSet %s failed"
)

var log = logf.Log.WithName(controllerName)

// PvcCleaner represents an object to clean Pvcs of a MysqlCluster
type PvcCleaner struct {
	cluster *wrapcluster.MysqlCluster
	opt     *options.Options
	sts     *apps.StatefulSet
}

// NewPvcCleaner returns a new PVC cleaner object
func NewPvcCleaner(cluster *api.MysqlCluster, opt *options.Options) *PvcCleaner {
	obj := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(api.StatefulSet),
			Namespace: cluster.Namespace,
		},
	}
	return &PvcCleaner{
		cluster: wrapcluster.NewMysqlClusterWrapper(cluster),
		opt:     opt,
		sts:     obj,
	}
}

// Run performs cleanup of orphaned pvcs in a Mysql cluster
func (p *PvcCleaner) Run(ctx context.Context, c client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) error {
	sts := p.sts
	stsMeta := sts.ObjectMeta

	key := types.NamespacedName{Name: stsMeta.GetName(), Namespace: stsMeta.GetNamespace()}

	err := c.Get(ctx, key, sts)
	if err != nil {
		return err
	}

	if sts.DeletionTimestamp != nil {
		log.V(4).Info("being deleted, no action", "Statefulset", key)
		return nil
	}

	if len(sts.Spec.VolumeClaimTemplates) == 0 {
		log.V(4).Info("ignored, no volume claims", "Statefulset", key)
		return nil
	}

	replicas := sts.Spec.Replicas

	// Find any pvcs with higher ordinal than replicas and delete them
	claims, err := p.getClaims(ctx, c)
	if err != nil {
		return err
	}

	keys := getKeys(claims)
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	for _, k := range keys {
		if int32(k) >= *replicas {
			log.Info("cleaning up", "pvc", claims[k])
			if err := deleteClaim(ctx, c, recorder, sts, claims[k]); err != nil {
				log.Error(err, "deleting claim")
			}
		}
	}
	return nil
}

func deleteClaim(ctx context.Context, c client.Client, recorder record.EventRecorder, sts *apps.StatefulSet, pvc *core.PersistentVolumeClaim) error {
	err := c.Delete(ctx, pvc)
	if err != nil {
		recorder.Event(sts, core.EventTypeWarning, deleteFail, fmt.Sprintf(messagePvcNotDeleted, pvc.Name, sts.Name))
		return err
	}

	recorder.Event(sts, core.EventTypeNormal, deleteSuccess, fmt.Sprintf(messagePvcDeleted, pvc.Name, sts.Name))
	return nil
}
func (p *PvcCleaner) getClaims(ctx context.Context, c client.Client) (map[int]*core.PersistentVolumeClaim, error) {
	sts := p.sts
	stsMeta := sts.ObjectMeta
	pvcs := &core.PersistentVolumeClaimList{}
	lo := &client.ListOptions{
		Namespace: stsMeta.GetNamespace(),
		Raw: &metav1.ListOptions{
			LabelSelector: getLabelString(p.cluster.GetLabels()),
		}}
	err := c.List(ctx, lo, pvcs)

	if err != nil {
		return nil, err
	}

	return byOrdinal(pvcs, sts), nil
}

func getLabelString(labels map[string]string) string {
	return fmt.Sprintf("app=%s,mysql_cluster=%s", labels["app"], labels["mysql_cluster"])
}

func byOrdinal(allClaims *core.PersistentVolumeClaimList) map[int]*core.PersistentVolumeClaim {
	claims := map[int]*core.PersistentVolumeClaim{}

	for _, pvc := range allClaims.Items {
		if pvc.DeletionTimestamp != nil {
			log.V(2).Info("being deleted, skipping", "pvc", pvc.Name)
			continue
		}

		_, ordinal, err := extract(pvc.Name)
		if err != nil {
			continue
		}

		claims[ordinal] = &pvc

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
