---
title: MySQL Cluster Setup
linktitle: Deploy
description: How to setup a cluster with MySQL operator.
keywords: [mysql operator, cluster]
menu:
  global:
    parent: "mysql-cluster"
    weight: 3
slug: deploy-mysql-cluster
---

How to deploy a MySQL cluster using the operator, after the operator is deployed as described in [getting started](./_index.md).

## Deploy a cluster

### Specify the cluster credentials

Before creating a cluster, you need a secret that contains the ROOT_PASSWORD key. An example for this secret can be found at
[examples/example-cluster-secret.yaml](https://github.com/presslabs/mysql-operator/blob/master/examples/example-cluster-secret.yaml).

Create a file named `example-cluster-secret.yaml` and copy into it the following YAML code:

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
data:
  # root password is required to be specified
  ROOT_PASSWORD: bm90LXNvLXNlY3VyZQ==
  # a user name to be created, not required
  USER: dXNlcm5hbWU=
  # a password for user, not required
  PASSWORD: dXNlcnBhc3Nz
  # a name for database that will be created, not required
  DATABASE: dXNlcmRi
```

This secret contains information about the credentials used to connect to the cluster, like `ROOT_PASSWORD`, `USER`,
`PASSWORD`, `DATABASE`. Note that once those fields are set, changing them will not reflect in the MySQL server because they are used only at cluster bootstrap.

> ###### NOTE
>
> All secret fields must be base64 encoded.

Moreover, the controller will add some extra fields into this secret with other internal credentials that are used, such as the orchestrator user, metrics exporter used, and so on.

### Create and deploy the cluster

Now, to create a cluster you need just a simple YAML file that defines it. Create a file named
`example-cluster.yaml` and copy into it the following YAML code:

``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  replicas: 2
  secretName: my-secret
```

A more comprehensive YAML example can be found at
[examples/examples-cluster.yaml](https://github.com/presslabs/mysql-operator/blob/master/examples/example-cluster.yaml).

> ###### NOTE
>
> Make sure that the cluster name is not too long, otherwise the cluster will fail to register with the orchestrator. See issue [#170](https://github.com/presslabs/mysql-operator/issues/170) to learn more.

To deploy the cluster, run the commands below which will generate a secret with credentials and the `MySQLCluster` resources into Kubernetes.

``` shell
$ kubectl apply -f example-cluster-secret.yaml
$ kubectl apply -f example-cluster.yaml
```

## Cluster configuration overview

Some important fields of `MySQLCluster` resource from `spec` are described in the following table:

| Field Name                       | Description                                                                              | Example                    | Default value           |
| ---                              | ---                                                                                      | ---                        | ---                     |
| `replicas`                       | The cluster replicas, how many nodes to deploy.                                          | 2                          | 1 (a single node)       |
| `secretName`                     | The name of the credentials secret. Should be in the same namespace with the cluster.    | `my-secret`                | *is required*           |
| `backupSchedule`                 | Represents the time and frequency of making cluster backups, in a cron format.           | `0 0 * * *`                | ""                      |
| `backupURL`                      | The bucket URL where to put the backup.                                                  | `gs://bucket/app`          | ""                      |
| `backupSecretName`               | The name of the secret that contains credentials for connecting to the storage provider. | `backups-secret`           | ""                      |
| `backupScheduleJobsHistoryLimit` | The number of many backups to keep.                                                      | `10`                       | inf                     |
| `mysqlConf`                      | Key-value configs for MySQL that will be set in `my.cnf` under `mysqld` section.         | `max_allowed_packet: 128M` | {}                      |
| `podSpec`                        | This allows to specify pod-related configs. (e.g. `imagePullSecrets`, `labels` )         |                            | {}                      |
| `volumeSpec`                     | Specifications for PVC, HostPath or EmptyDir, used to store data.                        |                            | (a PVC with size = 1GB) |
| `maxSlaveLatency`                | The allowed slave lag until it's removed from read service. (in seconds)                 | `30`                       | nil                     |
| `queryLimits`                    | Parameters for pt-kill to ensure some query run limits. (e.g. idle time)                 | `idelTime: 60`             | nil                     |
| `readOnly`                       | A Boolean value that sets the cluster in read-only state.                                | `True`                     | False                   |


For more detailed information about cluster structure and configuration fields can be found in [godoc](https://godoc.org/github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1#MysqlClusterSpec).

## Cluster data volume configuration

Currently, for a cluster generated with the MySQL operator you can set one of the following volume sources for MySQL data:

* Persistent Volume Claim
* Host Path (represents an existing directory on the host)
* Empty Dir (represents a temporary directory that shares a pod's lifetime).

### Persistent volume claim

An example to specify the PVC configuration:

``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  secretName: my-secret
  volumeSpec:
    persistentVolumeClaim:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi

```

If nothing is specified under the `volumeSpec` field, the cluster is created by default with a PVC of
`1Gi` in size, as described above. For more fine tuning of the PVC check the specs [godoc](https://godoc.org/k8s.io/api/core/v1#PersistentVolumeClaimSpec).

Note that the Kubernetes cluster needs to have a default storage class configured, otherwise it will fail to provision PVCs. If you want to change it, visit the [Kubernetes
documentation](https://kubernetes.io/docs/tasks/administer-cluster/change-default-storage-class/),
or just create a new storage class and specify it under `storageClassName` field in PVC specs.

> ###### NOTE
>
> Some additional information related to this topic can be found on issue
> [#168](https://github.com/presslabs/mysql-operator/issues/168).

### HostPath and EmptyDir

An example to use `HostPath` or `EmptyDir` as a volume source for data is:

``` yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  secretName: my-secret
  volumeSpec:
    hostPath:
      path: /path/to/host/dir/
    ## or use emptyDir
    # emptyDir: {}
```

For more configuration details regarding`hostPath` go [here](https://godoc.org/k8s.io/api/core/v1#HostPathVolumeSource), and more information about `emptyDir` can be found [here](https://godoc.org/k8s.io/api/core/v1#EmptyDirVolumeSource).

> ###### NOTE
>
> Note that EmptyDir is not fully supported from version `v0.3.x` of the operator. Please use `hostPath` instead. See information about the known [issue](https://github.com/presslabs/mysql-operator/issues/250).

## Cluster MySQL configuration

By default, the operator sets some sensible defaults on the cluster. Those configs can be found [here](https://github.com/presslabs/mysql-operator/blob/master/pkg/controller/mysqlcluster/internal/syncer/config_map.go#L146).
Also the `innodb-buffer-pool-size` and `innodb-log-file-size` are configured according to the requested memory, as described in this [article](https://www.percona.com/blog/2018/03/26/mysql-8-0-innodb_dedicated_server-variable-optimizes-innodb/).

The MySQL configs specified into `.spec.mysqlConf` have priority over the default values.
