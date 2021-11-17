---
title: Getting started with MySQL operator
linktitle: Getting started
description: The MySQL Kubernetes Operator manages all the necessary resources for deploying and managing a highly available MySQL cluster.
keywords: [mysql operator, cluster, getting-started]
---

## Getting started

Deploying and using the MySQL operator requires for it to have access to a Kubernetes cluster and to
have installed [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) and
[helm](https://github.com/helm/helm#install).

## Controller deploy

To deploy this controller, use the provided helm chart by running:

```shell
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator --name mysql-operator
```

For more information about chart values see chart
[README](https://github.com/presslabs/mysql-operator/blob/master/charts/mysql-operator/README.md).
This chart will deploy the controller along with an
[orchestrator](https://github.com/github/orchestrator) cluster.

__NOTE__: At every deploy a random password is generated for the orchestrator user. When running
`helm upgrade` this will change the password on the orchestrator side but not in the clusters and
this will break the communication between orchestrator and MySQL clusters. To solve this either use
`helm upgrade --reuse-values` or specify the orchestrator password. We recommend specifying the
orchestrator password.

## How to remove MySQL Operator completely from Kubernetes cluster to do a fresh deploy

Delete the helm release: 

```shell
helm -n <namespace> delete <operator-release>
```
And then delete the CRDs by running the following commands:

```shell
kubectl delete -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/mysql.presslabs.org_mysqlbackups.yaml
kubectl delete -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/mysql.presslabs.org_mysqlclusters.yaml 
kubectl delete -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/mysql.presslabs.org_mysqldatabases.yaml 
kubectl delete -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/mysql.presslabs.org_mysqlusers.yaml 
```

## Deploying a cluster

__tl;dr__

```shell
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster-secret.yaml
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster.yaml
```

#### More details

Before creating a cluster, you need a secret that contains the ROOT_PASSWORD key. An example for this secret can be found at
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

> ###### NOTE
>
> `ROOT_PASSWORD` must be base64 encoded.

Now, to create a cluster you need just a simple YAML file that defines it. An example can be found
at [examples/example-cluster.yaml](https://github.com/presslabs/mysql-operator/blob/master/examples/example-cluster.yaml).

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

For more in-depth configuration details, check these [examples](https://github.com/presslabs/mysql-operator/tree/master/examples).

### To list the deployed clusters use

```shell
$ kubectl get mysql
NAME         AGE
my-cluster   1m
```

### To check cluster state use

```shell
$ kubectl describe mysql my-cluster
```

``` shell
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

Backups are stored on object storage services like S3 or Google Cloud Storage. In order to be able to store a backup, the secret defined under `backupSecretName` must have the credentials to store those backups. The backups are uploaded using [Rclone](https://rclone.org/). The contents of the secret are used to generate a rclone.conf in [docker-entrypoint.sh](https://github.com/presslabs/mysql-operator/blob/master/hack/docker/sidecar-entrypoint.sh).

### Setup a backup on S3

You need to specify the `backupURL` for the cluster to an URL like `s3://BUCKET_NAME/cluster-name/`, and a secret.

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
  # AWS_STORAGE_CLASS: STANDARD
  # Optional, canned ACL to use
  # AWS_ACL:
  # Optional, the S3 provider to use (default: AWS)
  # S3_PROVIDER: AWS
  # Optional, the S3 endpoint to use (for when you use a different S3_PROVIDER)
  # S3_ENDPOINT:
```

Then run this command:

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
kubectl apply -f example-backup-secret.yaml
```

> ###### NOTE
>
> `GCS_SERVICE_ACCOUNT_JSON_KEY` and `GCS_PROJECT_ID` must be base64 encoded.

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
kubectl apply -f example-backup.yaml
```

You need to specify the `backupURL` for the corresponding cluster to an URL like
`gs://BUCKET_NAME/` and `backupSecretName`. Open the file named `example-cluster.yaml` and copy it into
 the following YAML code:

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
kubectl apply -f example-cluster.yaml
```

### Listing all backups

``` shell
$ kubectl get mysqlbackup
NAME                                  AGE
my-cluster-backup                     1m
my-cluster-auto-backup-20180402-1604  1d
```

### Checking the backup state

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

## Orchestrator

For more information about orchestrator and how to access it go to the [orchestrator](./orchestrator.md) section.
