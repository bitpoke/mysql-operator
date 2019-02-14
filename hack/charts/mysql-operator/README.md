# mysql-operator

This is the helm chart for [mysql-operator](https://github.com/presslabs/mysql-operator).

## TL;DR
```sh
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator --name mysql-operator
```

## Configuration
The following table contains the configuration parameters for mysql-operator and default values.

| Parameter                 | Description                                                                                   | Default value                             |
| ---                       | ---                                                                                           | ---                                       |
| `replicaCount`            | Replicas for controller                                                                       | `1`                                       |
| `image`                   | Controller container image                                                                    | `quay.io/presslabs/mysql-operator:v0.1.5` |
| `imagePullPolicy`         | Controller image pull policy                                                                  | `IfNotPresent`                            |
| `sidecarImage`            | Mysql operator sidecar image                                                                  | `quay.io/presslabs/mysql-helper:v0.1.5`   |
| `installCRDs`             | Whether or not to install CRDS                                                                | `true`                                    |
| `resources`               | Controller pod resources                                                                      | `{}`                                      |
| `nodeSelector`            | Controller pod nodeSelector                                                                   | `{}`                                      |
| `tolerations`             | Controller pod tolerations                                                                    | `{}`                                      |
| `affinity`                | Controller pod affinity                                                                       | `{}`                                      |
| `extraArgs`               | Args that are passed to controller, check controller command line flags                       | `[]`                                      |
| `watchNamespace`          | The namespace where the operator to watch for resources. Leave empty to watch all namespaces. | `""`                                      |
| `rbac.create`             | Whether or not to create rbac service account, role and roleBinding                           | `true`                                    |
| `rbac.serviceAccountName` | If `rbac.create` is false then this service account is used                                   | `default`                                 |
| `orchestrator.replicas`   | Control orchestraotr replicas                                                                 | `3`                                       |
| `orchestrator.image`      | Orchestrator container image                                                                  | `quay.io/presslabs/orchestrator:latest`   |
| `orchestrator.*`          | More ochestrator values that can be tuned. Check orchestrator chart values.yaml               |                                           |


For more orchestrator chart values check
[values.yaml](https://github.com/presslabs/docker-orchestrator/blob/master/charts/orchestrator/values.yaml)
from Orchestrator [chart](https://github.com/presslabs/docker-orchestrator/).
