# MySQL Operator

MySQL Operator is a bulletproof MySQL on Kubernetes. It manages all the necessary resources for a MySQL CLuster deployment. 

The MySQL Operator provides efortless backups, while the cluster being highly-available.

MySQL Operator was developed by the awesome engineering team at [Presslabs](https://www.presslabs.com/), 
a Managed WordPress Hosting provider.

For more open-source projects, check [Presslabs Code](https://www.presslabs.org/). 

## Goals

The main goals of this operator are:

1. Easily deploy mysql clusters in kubernetes (cluster-per-service model)
2. Friendly devops (monitoring story solved)
3. Out-of-the-box backups (automated and on demand) and point-in-time recovery
4. Support for cloning in cluster and across clusters

## Controller deploy

To deploy this controller, use the provided helm chart, by running:
```
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator --name mysql-operator
```

This chart will deploy the controller and an orchestrator cluster.


## Example of a cluster

Before creating a cluster, we need a secret that contains the `ROOT_PASSWORD` to
init mysql with (an example for this secret can be found
[here](examples/example-backup-secret.yaml)). Now, to create a cluster we need
just a simple yaml file that defines it — let's call it
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

For a more in depth configuration, check [examples](examples/). To deploy this
cluster, run:

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

Making a backup is easy — defining a cluster, you should define a backup;
for a detailed example, check [this](examples/example-backup.yaml) example. Let's
define a new one on:

```yaml
apiVersion: mysql.presslabs.net/v1alpha1
kind: MysqlBackup
metadata:
  name: foo-backup
spec:
  clusterName: foo
```

To deploy the backup, run:
```
kubeclt apply -f backup.yaml
```

To lists all backups, run:
```
$ kubectl get backup
NAME                             AGE
foo-backup                       1m
```

To check backup state and details, use:
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
...
```

## Access orchestrator
To connect to orchestrator dashboard you have to port forward orchestrator port
3000 to your local machine by using:

```
kubectl port-forward mysql-operator-orchestrator 3000
```


## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements
(eg. backup locks), monitoring improvements and some serviceability improvements
(eg. utility user). The case against MariaDB is that WordPress requires MySQL,
but MariaDB is not a drop-in replacement for it, since MariaDB 10 is a fork,
rather than following MySQL.

## License

This project is licensed under Apache 2.0 license. Read the [LICENSE](LICENSE) file in the
top distribution directory, for the full license text.
