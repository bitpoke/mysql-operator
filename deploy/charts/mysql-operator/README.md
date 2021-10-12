# mysql-operator

This is the helm chart for [mysql-operator](https://github.com/bitpoke/mysql-operator).

## TL;DR
```sh
## For Helm v2
helm repo add bitpoke https://helm-charts.bitpoke.io
helm install bitpoke/mysql-operator --name mysql-operator

## For Helm v3
helm repo add bitpoke https://helm-charts.bitpoke.io
helm install mysql-operator bitpoke/mysql-operator
```

## Configuration
The following table contains the configuration parameters for mysql-operator and default values.

| Parameter                       | Description                                                                                   | Default value                                           |
| ---                             | ---                                                                                           | ---                                                     |
| `replicas`                      | Replicas for controller                                                                       | `1`                                                     |
| `image`                         | Controller container image                                                                    | `docker.io/bitpoke/mysql-operator:latest`               |
| `imagePullPolicy`               | Controller image pull policy                                                                  | `IfNotPresent`                                          |
| `imagePullSecrets`              | Controller image pull secret                                                                  |                                                         |
| `sidecarImage`                  | Mysql operator sidecar image                                                                  | `docker.io/bitpoke/mysql-operator-sidecar-5.7:latest`   |
| `sidecarMysql8Image`            | Mysql operator sidecar image                                                                  | `docker.io/bitpoke/mysql-operator-sidecar-8.0:latest`   |
| `installCRDs`                   | Whether or not to install CRDS, Regardless of value of this, Helm v3+ will install the CRDs if those are not present already. Use `--skip-crds` with `helm install` if you want to skip CRD creation                                                                | `true`                                    |
| `resources`                     | Controller and Orchestrator pod resources limits and requests                                 | `{}`                                                    |
| `nodeSelector`                  | Controller and Orchestrator pod nodeSelector                                                  | `{}`                                                    |
| `tolerations`                   | Controller and Orchestrator pod tolerations                                                   | `{}`                                                    |
| `nodeAffinity`                  | Controller and Orchestrator pod affinity                                                      | `{}`                                                    |
| `antiAffinity`                  | To ensure 2 services don't end up on the same node can be one of `hard` or `soft`             | `hard`                                                  |
| `extraArgs`                     | Args that are passed to controller, check controller command line flags                       | `[]`                                                    |
| `watchNamespace`                | The namespace where the operator to watch for resources. Leave empty to watch all namespaces. | `""`                                                    |
| `rbac.create`                   | Whether or not to create rbac service account, role and roleBinding                           | `true`                                                  |
| `rbac.serviceAccountName`       | If `rbac.create` is false then this service account is used                                   | a newly created one or `default`                        |
| `podDisruptionBudget.enabled`   | Set PDB for Controller and Orchestrator nodes                                                 | `true`                                                  |
| `podSecurityPolicy.enabled`     | Enfoce pod security policy                                                                    | `false`                                                 |
| `orchestrator.image`            | Orchestrator container image                                                                  | `docker.io/bitpoke/mysql-operator-orchestrator:latest`  |
| `orchestrator.topologyUser`     | Set a user for orchestrator to use it to connect to the MySQL cluster                         | `orchestrator`                                          |
| `orchestrator.topologyPassword` | Set a password for the orchestrator user to connect to MySQL cluster                          | `<random string>`                                       |
| `orchestrator.persistence.fsGroupWorkaroundEnabled` | Adds a  workaround for persistent volume provisioners which don't support fsGroup (eg. when deploying to kind) [#615](https://github.com/bitpoke/mysql-operator/issues/615) | `false` |
| `orchestrator.*`                | More ochestrator values that can be tuned. Check in values.yaml                               |                                                         |
