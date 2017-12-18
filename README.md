# Titanium
Bulletproof mysql on kubernetes

## Goals
1. Easly deploy mysql clusters in kubernetes (cluster-per-service model)
2. Devops friendly (monitoring story solved)
3. Out of the box backups (automated and on demand) and point-in-time recovery
4. Support for clonning in cluster and across clusters

## Usage
### Backup on demand
Run `db backup` command on `pod-0`.
```
$ kubectl exec -it <release-name>-titanium-0 -- db backup
```
### Values
Value\_name   | Description | Default
--- | --- | ---
mysql.dbName    | Name of the db. (required) | ''
mysql.dbUser    | User name for db. (required) | ''
mysql.dbPassword | Password for db. (required) | ''
mysql.rootPassword | Root password. (required) | ''
--- | --- | ---
gsCredentialsFile | Service account json key to access backup. | _NULL_
initBucketURI     | URI for db initialization. | _NULL_
backupBucket      | Bucket for backups. | _NULL_
scheduleBackup    | Allows to schedule backup jobs. | "0 0 * * *"
--- | --- | ---
mysql.image | Mysql image. | percona:5.7
oysql.resources | Kubernetes resources per pod. | cpu: 200m, mem: 1Gi
mysql.persistence.storage | Size of PVC. | 8Gi
mysql.replicationUser | Replication user. | _(random string)_
mysql.replicationPassword | Replication password. | _(random string)_
persistenceDisabled | Used for testing. Disable PVC volumes. | _false_
--- | --- | ---
mysqlConfig | Object that forms the my.cnf file. | [values.yaml](https://github.com/PressLabs/titanium/blob/master/charts/titanium/values.yaml)

## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements (eg. backup locks), monitoring improvements and some serviceability improvements (eg. utility user). The case against MariaDB is that WordPress requires MySQL, but MariaDB is not a drop-in replacement for it. Since MariaDB 10 it's fork rather than following MySQL.
