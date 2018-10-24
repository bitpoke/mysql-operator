# mysql-operator

This is the helm chart for [mysql-operator](https://github.com/presslabs/mysql-operator).

## TL;DR
```sh
helm repo add presslabs https://presslabs.github.io/charts
helm install presslabs/mysql-operator --name mysql-operator
```

## Configuration
The following table contains the configuration parameters for mysql-operator and default values.

| Parameter                 | Description                                                                                                                                                | Default value                             |
| ---                       | ---                                                                                                                                                        | ---                                       |
| `replicaCount`            | replicas for controller                                                                                                                                    | `1`                                       |
| `image`                   | controller container image                                                                                                                                 | `quay.io/presslabs/mysql-operator:v0.1.5` |
| `imagePullPolicy`         | controller image pull policy                                                                                                                               | `IfNotPresent`                            |
| `sidecarImage`            | mysql operator sidecar image                                                                                                                               | `quay.io/presslabs/mysql-helper:v0.1.5`   |
| `installCRDs`             | whether or not to install CRDS                                                                                                                             | `true`                                    |
| `resources`               | controller pod resources                                                                                                                                   | `{}`                                      |
| `nodeSelector`            | controller pod nodeSelector                                                                                                                                | `{}`                                      |
| `tolerations`             | controller pod tolerations                                                                                                                                 | `{}`                                      |
| `affinity`                | controller pod affinity                                                                                                                                    | `{}`                                      |
| `extraArgs`               | args that are passed to controller                                                                                                                         | `[]`                                      |
| `rbac.create`             | whether or not to create rbac service account, role and roleBinding                                                                                        | `true`                                    |
| `rbac.serviceAccountName` | If `rbac.create` is false then this service account is used.                                                                                               | `default`                                 |
| `orchestrator.replicas`   | Control orchestraotr replicas                                                                                                                              | `3`                                       |
| `orchestrator.image`      | Orchestrator container image                                                                                                                               | `quay.io/presslabs/orchestrator:latest`   |
| `orchestrator.*`          | More ochestrator values that can be tuned. See [values.yaml](https://github.com/presslabs/docker-orchestrator/blob/master/charts/orchestrator/values.yaml) |                                           |
