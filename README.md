# MySQL Operator

MySQL Operator enables bulletproof MySQL on Kubernetes. It manages all the necessary resources for deploying and managing a highly available MySQL cluster. It provides effortless backups, while keeping the cluster highly available.

MySQL Operator is maintained and developed by [Bitpoke](https://www.bitpoke.io/), a self managed WordPress hosting app running cloud-native.

## Goals and status

The main goals of this operator are:

 1. Easily deploy MySQL clusters in Kubernetes (cluster-per-service model)
 2. Friendly to devops (monitoring, availability, scalability and backup stories solved)
 3. Out-of-the-box backups (scheduled and on demand) and point-in-time recovery
 4. Support for cloning in cluster and across clusters.


## Contributing

We welcome all contributions in the form of new issues for feature requests, bugs or even pull requests. We are open to discuss ideas on how to improve the operator and would also love to find out where and how it's used. The discussion related to this project should happen on the Kubernetes Community [Slack](https://kubernetes.slack.com/messages/CEKQXFR0E/). The current maintainers of this project can be reached via [email](mailto:hello@bitpoke.io), too.

## Documentation

* [Getting started](https://www.bitpoke.io/docs/mysql-operator/getting-started/) provides an overview over deploying and using the MySQL operator
* [Deploy a MySQL cluster](https://www.bitpoke.io/docs/mysql-operator/deploy-mysql-cluster/) describes in detail how a cluster can be installed and configured
* [Configure backups](https://www.bitpoke.io/docs/mysql-operator/backups/) shows how to configure and take backups of a cluster
* [Recurrent backups](https://www.bitpoke.io/docs/mysql-operator/cluster-recurrent-backups/) describes how to setup recurrent backups for the cluster
* [Restore a cluster](https://www.bitpoke.io/docs/mysql-operator/backups/#initialize-a-cluster-from-a-backup) explains how to restore a cluster from a backup
* [How to integrate](https://www.bitpoke.io/docs/mysql-operator/integrate-operator/) the operator with your deployment setup. This presents a simple way of using the MySQL operator and helm to deploy your application
* [Orchestrator](https://www.bitpoke.io/docs/mysql-operator/orchestrator/) shows you how to access the orchestrator for more details.

## Controller deploy

To deploy this controller, use the provided helm chart by running:

```shell
helm repo add bitpoke https://helm-charts.bitpoke.io
helm install mysql-operator bitpoke/mysql-operator
```

For more information about chart values see chart [README](deploy/charts/mysql-operator/README.md). This chart will deploy the controller together with an [orchestrator](https://github.com/github/orchestrator) cluster.

__NOTE__: MySQL operator 0.6.x requires at least Kubernetes 1.19.x to upgrade, check the [0.6.x upgrade notes](https://www.bitpoke.io/docs/mysql-operator/operator-upgrades/#v06x-upgrade) as some additional steps may be required.

__NOTE__: MySQL operator 0.2.x requires at least Kubernetes 1.11.x (or 1.10.x with alpha features) while version 0.1.x is known to work with Kubernetes up 1.9.x. To upgrade, check the [0.2.x upgrade notes](https://www.bitpoke.io/docs/mysql-operator/operator-upgrades/#v02x-upgrade) as some additional steps are required.

## Controller upgrade

Maybe upgrading the MySQL operator to a newer version requires additional steps. Those steps can be found in the operator's documentation at [upgrades](docs/operator-upgrades.md) section.

## Deploying a cluster


```shell
kubectl apply -f https://raw.githubusercontent.com/bitpoke/mysql-operator/master/examples/example-cluster-secret.yaml
kubectl apply -f https://raw.githubusercontent.com/bitpoke/mysql-operator/master/examples/example-cluster.yaml
```

## Roadmap to v1.0
 - [ ] Integration with Google Cloud Marketplace, OperatorHub.io, AWS Marketplace
 - [ ] CRD Validation and webhooks
 - [ ] SSL support
 - [ ] Default integrated with MySQL 8.0
 - [ ] Proxy SQL integration
 - [ ] Backup policies
 - [x] Physical backups
 - [x] HA [Orchestrator](https://github.com/github/orchestrator) integration
 - [x] [Prometheus](https://github.com/prometheus/prometheus) metrics exporter
 - [x] Lag mitigation
 - [x] Query limits


## Tech considerations

This project uses Percona Server for MySQL 5.7 / 8.0 because of backup improvements (eg. backup locks), monitoring improvements and some serviceability improvements (eg. utility user). Although we could have used MariaDB, our primary focus being WordPress, we wanted a drop-in rather than a fork. In the future we might support MariaDB if that can be implemented in a compatible way.

## License

This project is licensed under Apache 2.0 license. Read the [LICENSE](LICENSE) file in the top distribution directory for the full license text.
