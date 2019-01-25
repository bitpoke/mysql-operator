---
title: MySQL Operator recover
linktitle: Recurrent Backups and Initialization
description: MySQL Operator provides a way to recreate clusters from snapshots
categories: [mysql operator]
keywords: [mysql operator]
menu:
  docs:
    parent: "mysqloperator"
weight: 3
draft: false
aliases: []
toc: true
---

The MySQL Operator provides a way to recreate a cluster based on a backup (snapshot). Just create a
new cluster with a new name which has `iniBucketURL` field pointed to the right backup.

## Initialize a cluster from a backup

To initialize a new cluster from a backup just set the `initBucketURL` to the backup that you want
to use. The credentails for connecting to storage provider should be set in the secret specified
into `initBucketSecretName` field , the same as for `backupSecretName` presented in [Backup]({{< ref
"./backups.md" >}}) section. Those fields can point to the same secret.


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

This configuration gives you a new cluster that is initialized from the
specified backup `gs://bucket_name/path/to/backup.xbackup.gz`.
