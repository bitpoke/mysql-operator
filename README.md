# Titanium
Bulletproof mysql on kubernetes

## Goals
1. Easily deploy mysql clusters in kubernetes (cluster-per-service model)
2. Devops friendly (monitoring story solved)
3. Out of the box backups (automated and on demand) and point-in-time recovery
4. Support for cloning in cluster and across clusters

## Controller deploy
To deploy this controller use the provided helm chart, by running:
```
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator
```

This chart will deploy the controller and an orchestrator cluster.


## Example of a cluster
Before creating a cluster we need a secret that contains the `ROOT_PASSWORD` to
init mysql with (an example for this secret can be found
[here](examples/example-backup-secret.yaml)). Now to create a cluster we need
just a simple yaml file that defines it, let's call it
`cluster.yaml` and can be as follows:

```yaml
apiVersion: mysql.presslabs.net/v1alpha1
kind: MysqlCluster
metadata:
  name: foo
spec:
  replicas: 2
  secretName: the-secret
```

For a more in depth configuration checkout [examples](examples/). To deploy this
cluster run:

```
kubectl apply -f cluster.yaml
```

To list the deployed clusters use:
```
$ kubectl get mysql
NAME      AGE
foo       1m
```

To check cluster state use:
```
$ kubectl describe mysql foo
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

## Example of a backup
To make a backup it's easy, as defining a cluster you should define a backup,
for a detailed example check [this](examples/example-backup.yaml) example. Let's
define a new on:

```yaml
apiVersion: mysql.presslabs.net/v1alpha1
kind: MysqlBackup
metadata:
  name: foo-backup
spec:
  clusterName: foo
```

To deploy the backup run:
```
kubeclt apply -f backup.yaml
```

To lists all backups:
```
$ kubectl get backup
NAME                             AGE
foo-backup                       1m
```

To check backup state and details use:
```
$ kubectl describe backup foo-backup
...
Status:
  Completed:  true
  Conditions:
    Last Transition Time:  2018-03-21T16:02:56Z
    Message:               
    Reason:                
    Status:                True
    Type:                  Complete
    Last Transition Time:  2018-03-21T16:02:48Z
    Message:               First initialization of backup
    Reason:                set defaults
    Status:                Unknown
    Type:                  Failed
...
```

## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements
(eg. backup locks), monitoring improvements and some serviceability improvements
(eg. utility user). The case against MariaDB is that WordPress requires MySQL,
but MariaDB is not a drop-in replacement for it. Since MariaDB 10 it's fork
rather than following MySQL.
