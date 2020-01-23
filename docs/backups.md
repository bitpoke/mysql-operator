---
title: MySQL Cluster Backups
linktitle: Backups
description: MySQL operator provides effortless backups while keeping the cluster highly-available.
keywords: [mysql operator]
menu:
  global:
    parent: "mysql-cluster"
    weight: 1
slug: mysql-cluster-backups
---

Backups are stored on object storage services like S3 or Google Cloud Storage.

In order to be able to store a backup, the secret defined under `backupBucketSecretName` must have
the credentials to store those backups. The backups are uploaded using [Rclone](https://rclone.org/). The contents of the secret are used to generate a rclone.conf in [docker-entrypoint.sh](https://github.com/presslabs/mysql-operator/blob/master/hack/docker/sidecar-entrypoint.sh#L5).

### Setup a backup on S3

You need to specify the `backupBucketURL` for the cluster to an URL like `s3://BUCKET_NAME`, and a secret.

Create a file named `example-backup-secret.yaml` and copy into it the following YAML code:

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-cluster-backup-secret
type: Opaque
data:
  AWS_ACCESS_KEY_ID: #
  AWS_SECRET_ACCESS_KEY: #
  # Optional, the AWS region to connect
  # AWS_REGION: us-east1
  # Optional, specify the storage class
  # AWS_STORAGE_CLASS: standard
  # Optional, canned ACL to use
  # AWS_ACL:
  # Optional, the S3 provider to use (default: AWS)
  # S3_PROVIDER: AWS
  # Optional, the S3 endpoint to use (for when you use a different S3_PROVIDER)
  # S3_ENDPOINT:
```

Then run this command:

``` shell
$ kubectl apply -f example-backup-secret.yaml
```

### Setup a backup to Google Cloud

You need to specify the `backupBucketURL` for the cluster to an URL like `gs://BUCKET_NAME`, and a secret.

Create a file named `example-backup-secret.yaml` and copy into it the following YAML code:

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-cluster-backup-secret
type: Opaque
data:
  GCS_SERVICE_ACCOUNT_JSON_KEY: #
  GCS_PROJECT_ID: #
```

Then run this command:

``` shell
$ kubectl apply -f example-backup-secret.yaml
```

> ###### NOTE
>
> `GCS_SERVICE_ACCOUNT_JSON_KEY` and `GCS_PROJECT_ID` must be base64 encoded.

### Setup a backup to Azure Blob Storage

You need to specify the `backupBucketURL` for the cluster to an URL like `azure://STORAGE_ACCOUNT`, and a secret.

Create a file named `example-backup-secret.yaml` and copy into it the following YAML code:

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-cluster-backup-secret
type: Opaque
data:
  AZUREBLOB_ACCOUNT: #
  AZUREBLOB_KEY: #
```

Then run this command:

``` shell
$ kubectl apply -f example-backup-secret.yaml
```

> ###### NOTE
>
> `AZUREBLOB_ACCOUNT` and `AZUREBLOB_KEY` must be base64 encoded.

### Requesting a backup

You need to create a file named `example-backup.yaml` and copy into it the following YAML code:

``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlBackup
metadata:
  name: my-cluster-backup
spec:
  clusterName: my-cluster
```

Run the following command:

``` shell
$ kubectl apply -f example-backup.yaml
```

You need to specify the `backupBucketURL` for the corresponding cluster to an URL like `gs://BUCKET_NAME` and `backupSecretName`.
Open the file named `example-cluster.yaml` and copy into it the following YAML code:

``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  replicas: 2
  secretName: my-secret
  backupSecretName: my-cluster-backup-secret
  backupURL: gs://pl-test-mysql-operator/
```

Then run the following command:

``` shell
$ kubectl apply -f example-cluster.yaml
```

### Listing all backups

``` shell
$ kubectl get mysqlbackup
```

``` shell
NAME                                  AGE
my-cluster-backup                     1m
my-cluster-auto-backup-20180402-1604  1d
```

### Checking the backup state:

``` shell
$ kubectl describe backup my-cluster-backup
```

``` shell
...
Status:
  Completed:  true
  Conditions:
    Last Transition Time:  2018-03-21T16:02:56Z
    Message:               
    Reason:                
    Status:                True
    Type:                  Complete
...
```
