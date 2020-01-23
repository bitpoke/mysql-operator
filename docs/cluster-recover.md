---
title: MySQL Cluster Recovery
linktitle: Recovery
description: MySQL operator provides a way to recreate clusters from snapshots.
keywords: [mysql operator]
menu:
  global:
    parent: "mysql-cluster"
    weight: 2
slug: mysql-cluster-recovery
---

The MySQL operator provides a way to recreate a cluster based on a backup (snapshot). Just create a new cluster with a new name which has `initBucketURL` field pointed to the right backup.

## Initialize a cluster from a backup

To initialize a new cluster from a backup just set the `initBucketURL` to the backup that you want to use. The credentails for connecting to the storage provider should be set in the secret specified in the `initBucketSecretName` field , the same as for `backupSecretName` presented in the [Backup](./backups.md) section. These fields can point to the same secret.

``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  secretName: the-secret

  initBucketURL: gs://bucket_name/path/to/backup.xbackup.gz
  initBucketSecretName: backup-secret
```

This configuration gives you a new cluster that is initialized from the specified backup `gs://bucket_name/path/to/backup.xbackup.gz`.
