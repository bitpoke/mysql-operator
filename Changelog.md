# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [Unreleased]
### Added
 * Add `image` and `mysqlVersion` options to MysqlCluster chart. This bumps the chart version to `0.3.1`
 * Add `backupAffinity`, `backupNodeSelector`, `backupPriorityClassName`, `backupTolerations`
   to `.Spec.PodSpec` to allow specifying custom scheduling constraints for backup jobs.
 * Add the ability to set the `imagePullSecrets` for the operator statefulset.
 * Add Google Drive via service account as backup option.
 * Add `initBucketURL` and `initBucketSecretName` options to MysqlCluster chart. This bumps the chart version to `0.3.0`
 * Add an example of how initContainers can be used to fix hostPath permissions.
 * Add a lifecycle preStop hook for the `mysql` container. Before killing the master MySQL process,
   it triggers a `graceful-master-takeover-auto` command in Orchestrator. This is disabled by
   default, to enable it set `gracefulShutdown.enabled=true` in chart values or set the controller
   command argument `failover-before-shutdown` to `true`.
 * Add `mysqlLifecycle` to `.Spec.PodSpec` to allow overriding the default lifecycle hook for the
   `mysql` container.
 * Add `backupCompressCommand` and `backupDecompressCommand` to allow using
   different compressors/decompressors when backing up or restoring.
 * Add support for MySQL version 8.0
 * Add `go modules` cache
### Changed
 * Only add `binlog-space-limit` for `percona` image
 * Make user-defined InitContainer take the precedence
 * Set timeout of 15s on connection between the operator and Orchestrator
 * Bump controller-util dependency to 0.1.18 which fixes some updates on pod spec.
 * Removed `NO_AUTO_VALUE_ON_ZERO` from `sql-mode` to be inline with MySQL default value
 * Remove use go module instead of dep
 * Update k8s client to `v0.18.2`
 * Update kubebuilder (to `v2.3.1`) along with controller-runtime (to `v0.8.3`) and controller-gen
 * Update rclone to `v1.53.1`
 * Update `Orchestrator` version from `v3.1.2` to `v3.2.3`
 * Set default MySQL server version to `5.7.31`
 * Generate CRDs with controller-gen `v0.5.0`
 * Update client-go to `v0.20.4`
 * Update `getOrdinalFromHostname` and `IsFirstPodInSet`
 * Change use `klog`
### Removed
### Fixed
 * Fix insufficient permissions during startup
 * Fix the `xtrabackup` `--tables-exclude` cannot take effect
 * Fix the pod unable to connect `Orchestrator`
 * Fix pod labels diff of map
 * Fixed backup cleanup job bug (#577)
 * Fix Kubebuilder path in Makefile.
 * Fix #632 lifeCycle preStop script is not copied to given directory.
 * Fix #637 mysqlbackup status is not updated correctly.
 * Fix #647 custom conf can't overwrite the default conf
 * Fix #627 let Orchestrator do the failover
 * Fix #694 the error caused by backing up without waiting for master to be created.

## [0.4.0] - 2020-06-17
### Added
 * Added a `ServiceMonitor` to the operator chart that scrapes all MySQL instances.
 * Added a test suite for RunCloneCommand logic, along with a mock backup server.
 * Added checks for service availability when cloning.
 * Added "fail fast" logic when unexpected errors occur during cloning/download.
 * Added `dataDir` cleanup code so that interrupted cloning does not leave dataDir in an
   inconsistent state.
 * Added e2e test demonstrating cloning failure when PVC is removed and pod recreated.
 * Add `MetricsExporterExtraArgs` field on MySQLCluster resource that allows to specify command line
   arguments to pass to MySQL metrics exporter.
 * Allow using custom secret for app credentials in `mysql-cluster` chart.
 * Add `XbstreamExtraArgs` field on MySQLCluster resource that allows to specify extra command line
   arguments to xbstream.
 * Add `XtrabackupExtraArgs`, `XtrabackupPrepareExtraArgs`, `XtrabackupTargetDir` to parametrize
   xtrabackup.
 * Add `RcloneExtraArgs` to parametrize rclone command.
 * Add `InitFileExtraSQL` to insert custom init SQL queries that will be run at MySQL
   initialization.
 * Add `Volumes` and `VolumesMounts` in `.Spec.PodSpec` to allow the user specifying custom volume
   mounts.
 * Add `InitContainers` and `Containers` in `.Spec.PodSpec` to allow the user specifying custom containers.
 * Add `MetricsExporterResources` and `MySQLOperatorSidecarResrouces` in `.Spec.PodSpec` to allow
   the user specifying resources for thos sidecars containers.
 * Add command line flag to configure number of workers for orchestrator controller.
 * Add livenessProbe to controller
### Changed
 * [#422](https://github.com/presslabs/mysql-operator/pull/422) adds the `SidecarServerPort` to the
   `MasterService` and introduces one new service, HealthyReplicasService, so that we can try to
   clone from replicas first, then fall back to master.
 * Changed the connect timeout from the default of 30s to 5s so that an empty k8s service will not
   cause cloning attempts to hang unnecessarily for 30s.
 * Update documentation structure and formatting.
 * Update Orchestrator version to v3.1.4
 * Update orchestrator base image to `alpine:3.11`.
 * Update FailoverInProgress condition to false when both Replicas and ReadyNodes are 0.
 * Fall back to current master, not pod 0, when no healthy replicas found for backup candidate.
 * Change the `mysql-operator` chart to be helm v3 compatible while keeping backward compatibility.
 * Change logging: change `cluster` logging field to `key`; normalize logging and more details;
   output Stackdrive compatible format.
 * Decrease `MASTER_CONNECT_RETRY` interval from 10 to 1 second.
 * Deprecate `TmpfsSize` because can be handled using `ExtraVolumes` and `ExtraMySQLVolumesMounts`.
 * Update cron documentation
 * Set InnoDB buffer parameter: `innodb_buffer_pool_instances` to `min(resources.limit.cpu,
   floor(innodb_buffer_pool_size/1G))` (see #502)
 * Change default resource requests/limits for `sidecar` container: requested=10m/32Mi, limit=the same as `.spec.podSpec.resources.limit`
 * Change default resource requests/limits for `exporter` container: requested=10m/32Mi, limit=100m/128Mi
 * Change default resource requests/limits for `heartbeat` container: requested=10m/32Mi, limit=100m/64Mi
 * If [`extra_port`](https://www.percona.com/doc/percona-server/5.7/performance/threadpool.html#extra_port)
   is defined in the cluster spec, metrics exporter will use it to connect to MySQL providing that
   [`extra_max_connections`](https://www.percona.com/doc/percona-server/5.7/performance/threadpool.html#extra_max_connections)
   is larger than the default `1`. If MySQL server runs out of available connections, using `extra_port`
   allows the exporter to continue collecting MySQL metrics.
 * Change the default number of workers for orchestrator controller from 1 to 10.
### Removed
### Fixed
 * Update and fix e2e tests
 * Fix double date string in backup path
 * Fix double date string in bakup path
 * Copy the nodeSelector as-is in the statefulset (fixes #454)
 * Fix flakines in ReadOnly cluster condition (fixes #434)
 * Fix rounding in computing `innodb-buffer-pool-size` (fixes #501)
 * rclone extra arguments are now properly passed to the backup job.


## [0.3.8] - 2020-01-22
### Fixed
 * Fixed `TmpfsSize` type in CRD


## [0.3.7] - 2020-01-22
### Added
 * Add support for mounting a tmpfs into /tmp. Add `TmpfsSize` field on the cluster


## [0.3.6] - 2020-01-08
### Added
 * Update `mysql-cluster` chart to support setting `backupScheduleJobsHistoryLimit`
### Changed
 * Increase size of `value` column in `sys.operator` table (see
   [#447](https://github.com/presslabs/mysql-operator/pull/447#issuecomment-572538559)) (fixes #446)
 * Determine master logic: prevent to follow infinit loops
### Fixed
 * Use custom server offset (`MyServerIDOffset`) when deciding to clone from bucket


## [0.3.5] - 2019-11-28
### Changed
 * Update Orchestrator version to v3.1.2
 * Update Go lang to 1.13.4
### Fixed
 *  Don't require `backupSecretName` if not set. Fixes delete on remove when using workload identity


## [0.3.4] - 2019-11-04
### Added
 * Update `mysql-cluster` chart to support custom server id offset
### Changed
 * During failover don't interfere with readdable/wriatable master (fixes #411)
 * Imorove logging


## [0.3.3] - 2019-10-03
### Added
 * Allow server id offset to be set via `MySQLCluster` resource by adding `MyServerIDOffset` field
### Changed
 * Make app credentials optional for mysql-cluster chart
 * Don't create user with empty password (fixes #385)
### Fixed
 * Fix wrong enviroment for `mysql` container
 * Fix pt-kill client configure file prefix


## [0.3.2] - 2019-07-26
### Changed
 * Update docs structure on
   [presslabs.com/docs/mysql-operator/](https://www.presslabs.com/docs/mysql-operator/)
 * Set limit on mysql `mysql-init-only` container the same as on the `mysql` container (fixes #371)
 * Don't limit memory on sidecars containers


## [0.3.1] - 2019-07-17
### Fixed
 * Upgrade path from version `v0.2.x` (#369)
 * Changes the default leader election id from `controller-leader-election-helper` to
   `mysql-operator-leader-election` (#368)


## [0.3.0] - 2019-07-08
### Added
 * add a new cluster condition `FailoverInProgress` that marks the cluster during a failover
 * set orchestrator related events on the cluster: `OrcFailureDetection`,
   `OrcPostUnsuccessfulFailover`, `OrcPostMasterFailover`, `OrcPostIntermediateMasterFailover`
 * new command line flag `--mysql-versions-to-image` to allow user to specify image for a mysql
   version
 * add print column in mysqlcluster CRD: `Ready`, `Replicas`, `Age`
 * allow specifying more PodSpec on MySQL cluster (9b6b46f)
 * add a node controller for MySQL configuration (1950812)
 * add "standard" labels on services created by the operator (#299)
### Changed
 * merge Orchestrator chart with the mysql-operator chart. Now instead of deployment it uses a
   statefulset
 * nodes are removed from cluster status at scale down
 * use init container for MySQL initialization (#342)
 * enhance Backup Job Pod for Workload Identity (#366)
 * refactor of how the information flow works: from k8s -> Operator -> Orchestrator; the sidecar
   container does not connect to Orchestrator anymore.
 * rename of `initBackupURI` to `initBackupURL` (a3c6556)
 * use of Percona CentOS based images (#254)
 * don't run as `root` user in containers (#291)
 * rename orchestrator finalizer (to block cluster deletion while it's registered into Orchestrator)
   from `OrchestratorFinalizer` to `mysql.presslabs.org/registered-in-orchestrator` (bfe4646)
 * improvement of `getBackupCandidate` function (9ce4e68)
 * configure MySQL using `init-file` (beb41ce, e5823cb)
 * rename `AWS_SECRET_KEY` field from bucket/backup secret to `AWS_SECRET_ACCESS_KEY` (#301)
 * use an internal status table to store the MySQL status (181909f)
 * use a common headless service for all MySQL nodes to reduce the host-name length (#246)
### Removed
 * cleanup of deprecated fields: `bucketURI` and top-level `MySQLCluster.spec.volumeSpec` PVC
   specification (d909ab9, df80b28)
### Fixed
 * fix readiness probe for MySQL container
 * fix remote storage delete bug
 * fix mysqlbackup deletion failure when cluster is not found
 * fix #350 where no backups were created when one fails
 * fix #107, the orchestrator RAFT issue



[Unreleased]: https://github.com/presslabs/mysql-operator/compare/v0.4.0...HEAD
[0.3.8]: https://github.com/presslabs/mysql-operator/compare/v0.3.7...v0.3.8
[0.3.7]: https://github.com/presslabs/mysql-operator/compare/v0.3.6...v0.3.7
[0.3.6]: https://github.com/presslabs/mysql-operator/compare/v0.3.5...v0.3.6
[0.3.5]: https://github.com/presslabs/mysql-operator/compare/v0.3.4...v0.3.5
[0.3.4]: https://github.com/presslabs/mysql-operator/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/presslabs/mysql-operator/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/presslabs/mysql-operator/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/presslabs/mysql-operator/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/presslabs/mysql-operator/compare/v0.2.10...v0.3.0
[other]: https://github.com/presslabs/mysql-operator/releases?after=v0.3.0
