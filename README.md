# Titanium
Bulletproof mysql on kubernetes

## Goals
1. Easly deploy mysql clusters in kubernetes (cluster-per-service model)
2. Devops friendly (monitoring story solved)
3. Out of the box backups (automated and on demand) and point-in-time recovery
4. Support for clonning in cluster and across clusters

## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements (eg. backup locks), monitoring improvements and some serviceability improvements (eg. utility user). The case against MariaDB is that WordPress requires MySQL, but MariaDB is not a drop-in replacement for it. Since MariaDB 10 it's fork rather than following MySQL.
