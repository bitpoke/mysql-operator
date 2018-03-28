package backupscontroller

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	bfactory "github.com/presslabs/mysql-operator/pkg/backupfactory"
	controllerpkg "github.com/presslabs/mysql-operator/pkg/controller"
	"github.com/presslabs/mysql-operator/pkg/util"
	"github.com/presslabs/mysql-operator/pkg/util/options"
)

var (
	opt = options.GetOptions()
)

// Sync for add and update.
func (c *Controller) Sync(ctx context.Context, backup *api.MysqlBackup, ns string) error {
	glog.Infof("sync backup: %s", backup.Name)

	if len(backup.Spec.ClusterName) == 0 {
		return fmt.Errorf("cluster name is not specified")
	}

	if backup.Status.Completed {
		// silence skip it
		glog.V(2).Infof("Backup '%s' already competed, skiping.", backup.Name)
		return nil
	}

	cluster, err := c.clusterLister.MysqlClusters(backup.Namespace).Get(
		backup.Spec.ClusterName)
	if err != nil {
		return fmt.Errorf("cluster not found: %s", err)
	}

	copyBackup := backup.DeepCopy()
	factory := bfactory.New(copyBackup, c.k8client, c.tiClient, cluster)

	if err := factory.SetDefaults(); err != nil {
		return fmt.Errorf("set defaults: %s", err)
	}

	if err := factory.Sync(ctx); err != nil {
		return fmt.Errorf("sync: %s", err)
	}

	if _, err := c.tiClient.Mysql().MysqlBackups(ns).Update(copyBackup); err != nil {
		return fmt.Errorf("backup update: %s", err)
	}

	return nil
}

func (c *Controller) subresourceUpdated(obj interface{}) {
	var job *batch.Job
	var err error

	switch typedObject := obj.(type) {
	case *batch.Job:
		job = typedObject
	}

	if job == nil {
		glog.Errorf("Cannot get Job from object %#v", obj)
		return
	}

	backup, err := c.instanceForOwnerReference(&job.ObjectMeta)
	if err != nil {
		glog.Errorf("cannot get backup for Job, err: %s", err)
		return
	}

	key, err := controllerpkg.KeyFunc(backup)
	if err != nil {
		glog.Errorf("key func: %s", err)
		return
	}

	glog.V(2).Infof("Job '%s' is updated, requeueing backup: %s", job.Name, key)
	c.queue.Add(key)

	if i, exists := util.JobConditionIndex(batch.JobComplete, job.Status.Conditions); exists {
		cond := job.Status.Conditions[i]
		if cond.Status == core.ConditionTrue {
			// delete job after 5 hours
			key, err := controllerpkg.KeyFunc(job)
			if err != nil {
				glog.Errorf("key func: %s", err)
				return
			}
			glog.V(3).Infof("[subresourceUpdate] queueing '%s' job to deletion in %s.",
				key, opt.JobCompleteSuccessGraceTime)
			c.jobDeletionQueue.AddAfter(key, opt.JobCompleteSuccessGraceTime)
		}
	}
}
