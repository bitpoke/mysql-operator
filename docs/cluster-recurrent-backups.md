---
title: MySQL Operator Recurrent Backups
linktitle: Recurrent Backups
description: MySQL operator provides effortless recurrent backups while keeping the cluster highly available.
keywords: [mysql operator]
menu:
  global:
    parent: "mysql-operator"
    weight: 2
---

In the [Backups](./backups.md) section we take backups on demand. However, a cluster can be configured to take recurrent backups. Also, a cluster can be initialized from an existing backup.

To be able to store backups, the secret defined under `backupSecretName` must contain the credentials needed to access the storage provider(e.g GCS, AWS, etc.)

### Setup recurrent backups for MySQL cluster

You need to set the `backupBucketURL` in the cluster spec as an URL like `s3://BUCKET_NAME`, and the secret with storage credentials (`backupSecretName`).

See the example below to configure a cluster that has recurrent backups that runs once per day at midnight. To schedule a backup set `backupSchedule` field that is under crontab format. For more details about CRON format can be found [here](https://godoc.org/github.com/robfig/cron). Keep in mind this CRON format takes a 6th argument for seconds.

``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  secretName: the-secret

  backupSchedule: "0 0 0 * * *"  # daily
  backupURL: gs://bucket_name/path/
  backupSecretName: backup-secret
  backupRemoteDeletePolicy: retain|delete
```

Crontab takes 6 arguments from the traditional 5. The additional argument is a seconds field. Some crontab examples and their predefined schedulers:

| Entry         | Equivalent To          | Description                                |
| ------------- | -----                  | -----------                                |
| 15 0 0 1 1 *  | @yearly (or @annually) | Run once a year, midnight, Jan. 1st, 15th second        |
| 0 0 0 1 * *   | @monthly               | Run once a month, midnight, first of month, 0 second |
| 0 0 0 * * 0   | @weekly                | Run once a week, midnight between Sat/Sun, 0 second  |
| 0 0 0 * * *   | @daily (or @midnight)  | Run once a day, midnight, 0 second, 0 second                   |
| 0 0 * * * *   | @hourly                | Run once an hour, beginning of hour, 0 second        |


### Disable recurrent backups for MySQL cluster


``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  secretName: the-secret
  
  backupSchedule: ""  # set to empty string will disables recurrent backups
  backupURL: gs://bucket_name/path/
  backupSecretName: backup-secret
  backupRemoteDeletePolicy: retain|delete
```