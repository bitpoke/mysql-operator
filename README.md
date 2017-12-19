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
$ kubectl exec -it <release-name>-titanium-0 -c titanium -- db backup
```
### Configuration
Value name   | Description | Default
--- | --- | ---
mysql.dbName    | Name of the db. (required) | ''
mysql.dbUser    | User name for db. (required) | ''
mysql.dbPassword | Password for db. (required) | ''
mysql.rootPassword | Root password. (required) | ''
 | |
gsCredentialsFile | Service account json key to access backup. | _NULL_
initBucketURI     | URI for db initialization. | _NULL_
backupBucket      | Bucket for backups. (e.g. gs:<bucket\_name>) | _NULL_
backupPrefix      | Prefix where to put backups. | _(release name)_
scheduleBackup    | Allows to schedule backup jobs. | 0 0 * * * _(daily)_
 | |
mysql.image | Mysql image. | percona:5.7
mysql.resources | Kubernetes resources per pod. | cpu: 200m, mem: 1Gi
mysql.persistence.storage | Size of PVC. | 8Gi
mysql.replicationUser | Replication user. | _(random string)_
mysql.replicationPassword | Replication password. | _(random string)_
persistenceDisabled | Used for testing. Disable PVC volumes. | _false_
 | |
mysqlConfig | Object that forms the my.cnf file. | [values.yaml](https://github.com/PressLabs/titanium/blob/master/charts/titanium/values.yaml)

### Integration with other projects
The connection configuration is located in `{{ .Release.Name }}-db-credentials`
secret, that containes the following:

Key | Description
--- | ---
DB\_CONNECT\_URL | A url that contains all information needed to connect to db.
DB\_HOST | Hostname
DB\_NAME | Database name, same as _mysql.dbName_
DB\_USER | Database user, same as _mysql.dbUser_
DB\_PASSWORD | Password, same as _mysql.dbPassword_
DB\_PORT | Mysql port. (3306)


## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements (eg. backup locks), monitoring improvements and some serviceability improvements (eg. utility user). The case against MariaDB is that WordPress requires MySQL, but MariaDB is not a drop-in replacement for it. Since MariaDB 10 it's fork rather than following MySQL.
