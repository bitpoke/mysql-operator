# MySQL Operator

MySQL Operator enables bulletproof MySQL on Kubernetes. It manages all the necessary resources for
deploying and managing a highly available MySQL cluster. It provides efortless backups, while
keeping the cluster highly-available.

MySQL Operator was developed by the awesome engineering team at
[Presslabs](https://www.presslabs.com/), a Managed WordPress Hosting provider.

For more open-source projects, check [Presslabs Code](https://www.presslabs.com/code/).

## Goals and status

The main goals of this operator are:

 1. Easily deploy mysql clusters in kubernetes (cluster-per-service model)
 2. Friendly to devops (monitoring, availability, scalability and backup stories solved)
 3. Out-of-the-box backups (scheduled and on demand) and point-in-time recovery
 4. Support for cloning in cluster and across clusters

The operator is to be considered alpha and not suited for critical production workloads. We
(Presslabs) sucessfully use it at the moment for some non-critical production workloads.

## Contributing

We welcome all contributions in the form of new issues for feature requests, bugs or directly pull
requests. We are open to discuss ideas to improve the operator and would also love to find out where
and how it is used. The discussion related to the project should happen on Kubernetes Community
[Slack](https://kubernetes.slack.com/messages/CEKQXFR0E/). The current developers of the project can
be reached via [email](mailto:ping@presslabs.com) too.

## Documentation

More documentation can be found in [docs](docs/README.md)

## Controller deploy

To deploy this controller, use the provided helm chart, by running:

```shell
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator --name mysql-operator
```

For more information about chart values see chart [README](hack/charts/mysql-operator/README.md).
This chart will deploy the controller along with an
[orchestrator](https://github.com/github/orchestrator) cluster.

__NOTE__: MySQL Operator 0.2.x requires at least Kubernetes 1.11.x (or 1.10.x with alpha features)
while version 0.1.x is known to work with Kubernetes up 1.9.x. For upgrading check the [0.2.x
upgrade notes](#v02x-upgrade) since some manual seps are required.

## Controller upgrade

Maybe upgrading the MySQL operator to a newer version requires additional steps that has to be done.
Those steps can be found in the operator documentation at [upgrades](docs/operator-upgrades.md)
section.

## Deploying a cluster


```shell
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster-secret.yaml
kubectl apply -f https://raw.githubusercontent.com/presslabs/mysql-operator/master/examples/example-cluster.yaml
```

## Tech considerations

This project uses Percona Server for MySQL 5.7 because of backup improvements (eg. backup locks),
monitoring improvements and some serviceability improvements (eg. utility user). Although we could
have used MariaDB, our primary focus being WordPress, we wanted a drop-in rather than a fork. In the
future we might support MariaDB if that can be implemented in a compatible way.

## License

This project is licensed under Apache 2.0 license. Read the [LICENSE](LICENSE) file in the top
distribution directory, for the full license text.
