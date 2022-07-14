# Bitpoke MySQL Operator

This is the helm chart for [mysql-operator](https://github.com/bitpoke/mysql-operator).

## TL;DR
```sh
## For Helm v3
helm repo add bitpoke https://helm-charts.bitpoke.io
helm install mysql-operator bitpoke/mysql-operator
```

## Configuration
The following table contains the configuration parameters for mysql-operator and default values.

| Parameter                       | Description                                                                                   | Default value                                           |
| ---                             | ---                                                                                           | ---                                                     |
| `replicaCount`                  | Replicas for controller                                                                       | `1`                                                     |
| `image.repository`              | Docker repository for MySQL operator controller                                               | `docker.io/bitpoke/mysql-operator`                      |
| `image.tag`                     | Docker image tag for MySQL operator controller                                                | `latest`                                                |
| `image.pullPolicy`              | Docker image pull policy for MySQL operator controller                                        | `IfNotPresent`                                          |
| `sidecar57.image.repository`    | Docker repository for sidecar image for MySQL 5.7                                             | `docker.io/bitpoke/mysql-operator-sidecar-5.7`          |
| `sidecar57.image.tag`           | Docker image tag for sidecar image for MySQL 5.7                                              | `latest`                                                |
| `sidecar80.image.repository`    | Docker repository for sidecar image for MySQL 8.0                                             | `docker.io/bitpoke/mysql-operator-sidecar-8.0`          |
| `sidecar80.image.tag`           | Docker image tag for sidecar image for MySQL 8.0                                              | `latest`                                                |
| `metricsExporter.image.repository`    | Docker repository for MySQL metrics exporter                                            | `docker.io/prom/mysqld-exporter`                        |
| `metricsExporter.image.tag`           | Docker image tag for MySQL metrics exporter                                             | `v0.13.0`                                                      |
| `imagePullSecrets`              | Controller image pull secrets                                                                 | `[]`                                                    |
| `extraEnv`                      | MySQL Operator controller extra environment variables                                         | `[]`                                                    |
| `extraArgs`                     | MySQL Operator controller extra command line arguments                                        | `[]`                                                    |
| `resources`                     | MySQL Operator controller resources limits and requests                                       | `{}`                                                    |
| `rbac.create`                   | Specifies whether RBAC objects like `Role` and `RoleBinding` should be created                | `true`                                                  |
| `serviceAccount.create`         | Specifies whether a service account should be created                                         | `true`                                                  |
| `serviceAccount.annotations`    | Annotations to add to the service account                                                     | `{}`                                                    |
| `serviceAccount.name`           | The name of the service account to use. If not set and create is true, a name is generated using the fullname template. | `empty`                       |
| `podAnnotations`                | Extra pod annotations                                                                         | `{}`                                                    |
| `podSecurityContext`            | The pod security context. `65532` is the UID/GID for the nonroot user in the official images  | `{runAsNonRoot: true, runAsUser: 65532, runAsGroup: 65532, fsGroup: 65532}` |
| `securityContext`               | Security context for the MySQL Operator container                                             | `{}`                                                    |
| `podDisruptionBudget.enabled`   | Specifies whether a PodDisruptionBudget for the MySQL operator StatefulSet should be created. <br /> _NOTE_ it only takes effect if `replicaCount` is greated than 1. | `true` |
| `podDisruptionBudget.minAvailable`   | ref: https://kubernetes.io/docs/tasks/run-application/configure-pdb/#specifying-a-poddisruptionbudget | `empty` |
| `podDisruptionBudget.maxUnavailable` | ref: https://kubernetes.io/docs/tasks/run-application/configure-pdb/#specifying-a-poddisruptionbudget | `1`     |
| `podSecurityPolicy.enabled`     | Specified whether a PodSecurityPolicy should be created. PSP are deprecated and this will be removed in the future | `false` |
| `nodeSelector`                  | MySQL Operator pod nodeSelector                                                               | `{}`                                                    |
| `tolerations`                   | MySQL Operator pod tolerations                                                                | `[]`                                                    |
| `affinity`                      | MySQL Operator pod affinity                                                                   | `{}`                                                    |
| `gracefulShutdown.enabled`      | Insert a pre-stop lifecycle hook and trigger a failover when a MySQL pod is stopped           | `true`                                                  |
| `watchNamespace`                | The namespace where the operator to watch for resources. Leave empty to watch all namespaces. | `empty`                                                 |
| `orchestrator.image.repository` | Docker repository for [orchestrator](https://github.com/openark/orchestrator)                 | `docker.io/bitpoke/mysql-operator-orchestrator`         |
| `orchestrator.image.tag`        | Docker image tag for [orchestrator](https://github.com/openark/orchestrator)                  | `latest`                                                |
| `orchestrator.image.pullPolicy` | Docker image pull policy for [orchestrator](https://github.com/openark/orchestrator)          | `IfNotPresent`                                          |
| `orchestrator.topologyUser`     | Set a user for orchestrator to use it to connect to the MySQL cluster                         | `orchestrator`                                          |
| `orchestrator.topologyPassword` | Set a password for the orchestrator user to connect to MySQL cluster                          | `<random string>`                                       |
| `orchestrator.persistence.enabled` | Specifies whether to enable the persistence for Orchestrator pods                          | `true`                                                  |
| `orchestrator.persistence.*`    | Check `values.yaml`                                                                           |                                                         |
| `orchestrator.ingress.enabled`  | Specifies whether to enable the ingress for Orchestrator                                      | `false`                                                 |
| `orchestrator.ingress.*`        | Check `values.yaml`                                                                           |                                                         |
| `orchestrator.service.*`        | Orchestrator `Service` definition. Check `values.yaml`                                        |                                                         |
| `orchestrator.persistence.fsGroupWorkaroundEnabled` | Adds a  workaround for persistent volume provisioners which don't support fsGroup (eg. when deploying to kind) [#615](https://github.com/bitpoke/mysql-operator/issues/615) | `false` |
