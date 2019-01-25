---
title: MySQL cluster setup
linktitle: MySQL Operator Integration
description: How to setup a cluster with MySQL Operator
categories: [mysql operator, cluster]
keywords: [mysql operator, cluster]
menu:
  docs:
    parent: "mysqloperator"
weight: 3
draft: false
aliases: []
toc: true
---

How to deploy a MySQL cluster with the operator. After the Operator is deployed
as describe in [getting started]({{< ref "./getting-started.md" >}}).

## Deploy a cluster

### Specify the cluster credentials

Before creating a cluster, you need a secret that contains the ROOT_PASSWORD key. An example for
this secret can be found at
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

This secret contains information about cluster connect credentials like `ROOT_PASSWORD`, `USER`,
`PASSWORD`, `DATABASE`. Note that, once those fields are set changing them will not reflect in
MySQL server, because are use only at cluster bootstrap.

> All secret fields must be base64 encoded.

Moreover the controller will add some extra fields into this secret with other internal credentials
that are used like orhestrator user, metrics exporter used, and so on.

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

> Make sure that the cluster name is not too long, otherwise the cluster will fail to register with
> orchestrator. See issue [#170](https://github.com/presslabs/mysql-operator/issues/170) about this.

To deploy the cluster, run the commands from below, that creates the credentails secret and the
MySQLCluster resources into Kubernetes.

``` shell
$ kubectl apply -f example-cluster-secret.yaml
$ kubectl apply -f example-cluster.yaml
```

## Cluster configuration overview

Some important fields of `MysqlCluster` resource from `spec` are described in the following table.

| Field Name                       | Description                                                                              | Example                    | Default value           |
| ---                              | ---                                                                                      | ---                        | --                      |
| `replicas`                       | The cluster replicas, how many nodes to be deployed.                                     | 2                          | 1 (a single node)       |
| `secretName`                     | Is the name of the credentials secret. Should be in the same namespace with the cluster. | `my-secret`                | *is required*           |
| `backupSchedule`                 | Represents the time and frequency of doing cluster backups, as a cron format.            | `0 0 * * *`                | ""                      |
| `backupURL`                      | The bucket URL where to put the backup.                                                  | `gs://bucket/app`          | ""                      |
| `backupSecretName`               | The name of the secret that contains credentials for connecting to the storage provider. | `backups-secret`           | ""                      |
| `backupScheduleJobsHistoryLimit` | How many backups to be kept.                                                            | `10`                       | inf                     |
| `mysqlConf`                      | Key-value configs for MySQL that will be set in `my.cnf` under `mysqld` section.         | `max_allowed_packet: 128M` |                         |
| `podSpec`                        | This allows to specify pod related configs. (e.g. `imagePullSecrets`, `labels` )         |                            | nil                     |
| `volumeSpec`                     | Specification for PVC, HostPath or EmptyDir that will be used to store data.             |                            | (a PVC with size = 1GB) |
| `maxSlaveLatency`                | Is the allowed slave lag until it's removed from read service. (in seconds)              | `30`                       | nil                     |
| `queryLimits`                    | Parameters for pt-kill to ensure some query run limits. (e.g. idle time)                 | `idelTime: 60`             | nil                     |
| `readOnly`                       | A Boolean value that sets the cluster in read only state.                                | `True`                     | False                   |


For more detailed information about cluster structure and configuration fields can be found in
[godoc](https://godoc.org/github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1#MysqlClusterSpec).

## Cluster data volume configuration

Currently, for a cluster generated with the MySQL Operator it can be set one of the following volume
source for MySQL data:
 * Persistent Volume Claim
 * Host Path (represents an existing directory on the host)
 * Empty Dir (represents a temporary directory that shares a pod's lifetime)

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

If nothing is specified under `volumeSpec` field, the cluster, by default, is created with a PVC of
`1Gi` in size, as described above. For more fine tuning of the PVC check PVC spec
[godoc](https://godoc.org/k8s.io/api/core/v1#PersistentVolumeClaimSpec).

Note that: the k8s cluster needs to have a default storage class configured, otherwise will fail
provisioning PVCs. If you want to change it, see [k8s
documentation](https://kubernetes.io/docs/tasks/administer-cluster/change-default-storage-class/),
or just create a new storage class and specify it under `storageClassName` field in PVC specs.

> Some information related to this can be found on issue
> [#168](https://github.com/presslabs/mysql-operator/issues/168).

### HostPath and EmptyDir

An example to use `HostPath` or `EmptyDir` as volume source for data is:

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

For more configuration of `hostPath` can be found
[here](https://godoc.org/k8s.io/api/core/v1#HostPathVolumeSource), and more information about
`emptyDir` can be found [here](https://godoc.org/k8s.io/api/core/v1#EmptyDirVolumeSource).


## Cluster MySQL configuration

By default the operator sets some sensible defaults on the cluster. Those configs can be found
[here](https://github.com/presslabs/mysql-operator/blob/master/pkg/controller/mysqlcluster/internal/syncer/config_map.go#L146).
Also the `innodb-buffer-pool-size` and `innodb-log-file-size` are configured accordingly with the
requested memory as described in this
[article](https://www.percona.com/blog/2018/03/26/mysql-8-0-innodb_dedicated_server-variable-optimizes-innodb/).

The MySQL configs specified into `.spec.mysqlConf` have priority over the default values.
