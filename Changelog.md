# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [Unreleased]
### Added
### Changed
### Removed
### Fixed


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
 * Increase size of `value` column in `sys.operator` table (see [#447](https://github.com/presslabs/mysql-operator/pull/447#issuecomment-572538559)) (fixes #446)
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
 * Update docs structure on [presslabs.com/docs/mysql-operator/](https://www.presslabs.com/docs/mysql-operator/)
 * Set limit on mysql `mysql-init-only` container the same as on the `mysql` container (fixes #371)
 * Don't limit memory on sidecars containers


## [0.3.1] - 2019-07-17
### Fixed
 * Upgrade path from version `v0.2.x` (#369)
 * Changes the default leader election id from `controller-leader-election-helper` to `mysql-operator-leader-election` (#368)


## [0.3.0] - 2019-07-08
### Added
 * add a new cluster condition `FailoverInProgress` that marks the cluster during a failover 
 * set orchestrator related events on the cluster: `OrcFailureDetection`, `OrcPostUnsuccessfulFailover`, `OrcPostMasterFailover`, `OrcPostIntermediateMasterFailover`
 * new command line flag `--mysql-versions-to-image` to allow user to specify image for a mysql version
 * add print column in mysqlcluster CRD: `Ready`, `Replicas`, `Age` 
 * allow specifying more PodSpec on MySQL cluster (9b6b46f)
 * add a node controller for MySQL configuration (1950812)
 * add "standard" labels on services created by the operator (#299)
### Changed
 * merge Orchestrator chart with the mysql-operator chart. Now instead of deployment it uses a statefulset
 * nodes are removed from cluster status at scale down
 * use init container for MySQL initialization  (#342)
 * enhance Backup Job Pod for Workload Identity (#366) 
 * refactor of how the information flow works: from k8s -> Operator -> Orchestrator;  the sidecar container does not connect to Orchestrator anymore.
 * rename of `initBackupURI` to `initBackupURL` (a3c6556)
 * use of Percona CentOS based images (#254)
 * don't run as `root` user in containers (#291)
 * rename orchestrator finalizer (to block cluster deletion while it's registered into Orchestrator) from `OrchestratorFinalizer` to `mysql.presslabs.org/registered-in-orchestrator` (bfe4646)
 * improvement of `getBackupCandidate` function (9ce4e68)
 * configure MySQL using `init-file` (beb41ce, e5823cb)
 * rename `AWS_SECRET_KEY` field from bucket/backup secret to `AWS_SECRET_ACCESS_KEY` (#301) 
 * use an internal status table to store the MySQL status (181909f)
 * use a common headless service for all MySQL nodes to reduce the host-name length (#246)
### Removed
 * cleanup of deprecated fields: `bucketURI` and top-level `MySQLCluster.spec.volumeSpec` PVC specification (d909ab9, df80b28)
### Fixed
 * fix readiness probe for MySQL container
 * fix remote storage delete bug
 * fix mysqlbackup deletion failure when cluster is not found
 * fix #350 where no backups were created when one fails
 * fix #107, the orchestrator RAFT issue



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
