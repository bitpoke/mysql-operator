__NOTE__: MySQL Operator 0.2.x requires at least Kubernetes 1.11.x (or 1.10.x with alpha features) while version 0.1.x is known to work with Kubernetes up 1.9.x. For upgrading check the [0.2.x upgrade notes](#v02x-upgrade) since some manual seps are required.

# MySQL Operator

MySQL Operator enables bulletproof MySQL on Kubernetes. It manages all the necessary resources for deploying and managing a highly available MySQL cluster. It provides efortless backups, while keeping the cluster highly-available.

MySQL Operator was developed by the awesome engineering team at [Presslabs](https://www.presslabs.com/), 
a Managed WordPress Hosting provider.

For more open-source projects, check [Presslabs Code](https://www.presslabs.org/).

## Goals and status

The main goals of this operator are:

 1. Easily deploy mysql clusters in kubernetes (cluster-per-service model)
 2. Friendly to devops (monitoring, availability, scalability and backup stories solved)
 3. Out-of-the-box backups (scheduled and on demand) and point-in-time recovery
 4. Support for cloning in cluster and across clusters

The operator is to be considered alpha and not suited for critical production workloads. We (Presslabs) sucessfully use it at the moment for some non-critical production workloads.

## Contributing

We welcome all contributions in the form of new issues for feature requests, bugs or directly pull requests.
We are open to discuss ideas to improve the operator and would also love to find out where and how it is used.
The discussion related to the project should happen on [Gitter](https://gitter.im/PressLabs/mysql-operator).
The current developers of the project can be reached via [email](mailto:ping@presslabs.com) too.

## Controller deploy

Install helm. New to helm? Check https://github.com/helm/helm#install 

Install kubectl. For more details, see: https://kubernetes.io/docs/tasks/tools/install-kubectl/ 

To deploy this controller, use the provided helm chart, by running:
```shell
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator --name mysql-operator
```

For more information about chart values see chart [README](hack/charts/mysql-operator/README.md).
This chart will deploy the controller along with an [orchestrator](https://github.com/github/orchestrator) cluster.

### v0.2.x upgrade

1. Scale to 0 the current operator deployment. This won't affect your running databases.
2. Upgrade your cluster control plane to 1.11
3. Update the installed CRDs: `kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/hack/02x-crds.yaml`
4. Upgrade the mysql-operator `helm upgrade mysql-operator presslabs/mysql-operator`
5. Scale the operator deployment up
6. Now you can upgrade you nodes as well.
7. Enjoy!

## Deploying a cluster
__tl;dr__


```shell
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster-secret.yaml
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster.yaml
```

#### More details:

Before creating a cluster, you need a secret that contains the ROOT_PASSWORD key.
An example for this secret can be found at [examples/example-cluster-secret.yaml](examples/example-cluster-secret.yaml).

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

Now, to create a cluster you need just a simple YAML file that defines it.
An example can be found at [examples/example-cluster.yaml](examples/example-cluster.yaml). 

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

For a more in depth configuration, check [examples](examples/).

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

Backups are stored on object storage services like S3 or google cloud storage.
In order to be able to store backup, the secret defined under `backupBucketSecretName` must the credentials to store those backups.
The backups are uploaded using [Rclone](https://rclone.org/).
The contents of the secret are used to generate an rclone.conf in [hack/docker/mysql-helper/docker-entrypoint.sh](hack/docker/mysql-helper/docker-entrypoint.sh).

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

You need to specify the `backupBucketUri` for the corresponding cluster to an URI like `gs://BUCKET_NAME` and `backupSecretName`.
Open the file named `example-cluster.yaml` and copy into it the following YAML code:

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

## Access orchestrator
To connect to orchestrator dashboard you have to port forward orchestrator port
3000 to your local machine. Ensure it's a healthy pod if using raft:

```shell
kubectl port-forward mysql-operator-orchestrator-0 3000
```

Then type `localhost:3000` in a browser.

## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements
(eg. backup locks), monitoring improvements and some serviceability improvements
(eg. utility user). Although we could have used MariaDB, our primary focus being WordPress,
we wanted a drop-in rather than a fork. In the future we might support MariaDB if that can
be implemented in a compatible way.

## License

This project is licensed under Apache 2.0 license. Read the [LICENSE](LICENSE) file in the
top distribution directory, for the full license text.
