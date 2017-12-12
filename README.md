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
Value_name   | Description
--- | ---
db.name    | Name of the db.
db.user    | User name for db.
db.password | Password for db.
dsCredentialsFile | Service account json key to access backup.
initBucketURI     | URI for db initialization.
backupBucket      | Bucket for backups.
scheduleBackup    | Allows to schedule backup jobs.
mysql.image | Mysql image (mysql:5.7).
mysql.replicas | Number of replicas.
mysql.resources | Kubernetes resources per pod.
mysql.persistence.storage | Size of PVC. (8Gi)
mysql.replication.user | Replication user.
persistenceDisabled | Used for testing. Disable PVC volumes.

## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements (eg. backup locks), monitoring improvements and some serviceability improvements (eg. utility user). The case against MariaDB is that WordPress requires MySQL, but MariaDB is not a drop-in replacement for it. Since MariaDB 10 it's fork rather than following MySQL.
