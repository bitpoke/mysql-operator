---
title: Getting started
linktitle: Getting started with MySQL Operator
description: How to setup a cluster with MySQL Operator
categories: [mysql operator]
keywords: [mysql operator, cluster, getting-started]
menu:
  docs:
    parent: "mysqloperator"
weight: 3
draft: false
aliases: []
toc: true
---

For deploying and using MySQL operator it needs to have access to a Kubernetes cluster and to have
installed [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) and
[helm](https://github.com/helm/helm#install).

## Controller deploy

To deploy this controller, use the provided helm chart, by running:

```shell
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator --name mysql-operator
```

For more information about chart values see chart [README](hack/charts/mysql-operator/README.md).
This chart will deploy the controller along with an [orchestrator](https://github.com/github/orchestrator) cluster.

## Deploying a cluster
__tl;dr__

```shell
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster-secret.yaml
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster.yaml
```

#### More details:

Before creating a cluster, you need a secret that contains the ROOT_PASSWORD key. An example for
this secret can be found at
[examples/example-cluster-secret.yaml](https://github.com/presslabs/mysql-operator/blob/master/examples/example-cluster-secret.yaml).

Create a file named `example-cluster-secret.yaml` and copy into it the following YAML code:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
data:
  # root password is required to be specified
  ROOT_PASSWORD: bm90LXNvLXNlY3VyZQ==
```
#### Note. 
`ROOT_PASSWORD` must be base64 encoded.

Now, to create a cluster you need just a simple YAML file that defines it. An example can be found
at
[examples/example-cluster.yaml](https://github.com/presslabs/mysql-operator/blob/master/examples/example-cluster.yaml).

Create a file named `example-cluster.yaml` and copy into it the following YAML code:

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  replicas: 2
  secretName: my-secret
```

To deploy the cluster, run: 

```shell
kubectl apply -f example-cluster-secret.yaml
kubectl apply -f example-cluster.yaml
```

For a more in depth configuration, check
[examples](https://github.com/presslabs/mysql-operator/tree/master/examples).

### To list the deployed clusters use
```shell
$ kubectl get mysql
NAME         AGE
my-cluster   1m
```

### To check cluster state use
```shell
$ kubectl describe mysql my-cluster
...
Status:
  Ready Nodes:  2
  Conditions:
    Last Transition Time:  2018-03-28T10:20:23Z
    Message:               Cluster is ready.
    Reason:                statefulset ready
    Status:                True
    Type:                  Ready
...
```

## Backups

Backups are stored on object storage services like S3 or google cloud storage. In order to be able
to store backup, the secret defined under `backupBucketSecretName` must the credentials to store
those backups. The backups are uploaded using [Rclone](https://rclone.org/). The contents of the
secret are used to generate an rclone.conf in
[docker-entrypoint.sh](https://github.com/presslabs/mysql-operator/blob/master/hack/docker/sidecar-entrypoint.sh).

### Setup backup to S3

You need to specify the `backupBucketUri` for the cluster to an uri like `s3://BUCKET_NAME`, and a secret.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-cluster-backup-secret
type: Opaque
data:
  AWS_ACCESS_KEY_ID: #
  AWS_SECRET_KEY: #
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

```yaml
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

```shell
kubectl apply -f example-backup-secret.yaml
```

#### Note
`GCS_SERVICE_ACCOUNT_JSON_KEY` and `GCS_PROJECT_ID` must be base64 encoded.

### Requesting a backup

You need to create a file named `example-backup.yaml` and copy into it the following YAML code:

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlBackup
metadata:
  name: my-cluster-backup
spec:
  clusterName: my-cluster
```

Run the following command:

```shell
kubectl apply -f example-backup.yaml 
```

You need to specify the `backupBucketUri` for the corresponding cluster to an URI like
`gs://BUCKET_NAME` and `backupSecretName`. Open the file named `example-cluster.yaml` and copy into
it the following YAML code:

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  replicas: 2
  secretName: my-secret
  backupSecretName: my-cluster-backup-secret
  backupUri: gs://pl-test-mysql-operator/
```

Then run the following command: 

```shell
kubectl apply -f example-cluster.yaml
```

### Listing all backups
```shell
$ kubectl get mysqlbackup
NAME                                  AGE
my-cluster-backup                     1m
my-cluster-auto-backup-20180402-1604  1d
```

### Checking the backup state
```shell
$ kubectl describe backup my-cluster-backup
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

## Orchestrator

For more information about Orchestrator and how to access it go to
[Orchestrator](docs/orchestrator.md) section.
