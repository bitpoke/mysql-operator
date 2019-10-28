---
title: Integrating mysql clusters into your own helm charts
linktitle: MySQL Operator Integration
description: How to integrate the MySQL operator with your application.
categories: [mysql operator]
keywords: [mysql operator]
toc: true
related: true
slug: integrate-operator
---

After cluster creation, you can update the provided secret with a new field named `DB_CONNECT_URL`
that contains a [DSN](https://en.wikipedia.org/wiki/Data_source_name) to connect to the writable
cluster endpoint. You can check the `_helper.tpl` file for more insights.

The MySQL operator provides 3 services to access the nodes:

* `<cluster_name>-mysql-master` is the service that points to the master node and this endpoint
 should be used for writes. This service is usually used to construct the DSN.
* `<cluster_name>-mysql` is the service that routes traffic to _all healthy_ nodes from the
 cluster. You should use this endpoint for reads.
* `mysql` is the service used internally to access all nodes within a namespace. You can use this
 service to access a specific node (e.g. `<cluster_name>-mysql-0.mysql.<namespace>`)

We use helm to deploy our application into Kubernetes, so we updated our charts to use the MySQL
operator to provide one cluster per application.

## Using Helm

Usually a cluster of MySQL is needed alongside with an application, that's why we provide a Helm
chart for easy deployment of a MySQL cluster. The chart can be found in `presslabs` chart repository and
installed like the operator. Below is illustrated how this chart can be integrated with your
application to provision a MySQL cluster.

### Add MySQL Cluster chart as dependency

In your chart add in `requirements.yaml` under `dependencies` section the following:

```yaml
dependencies:
  - name: mysql-cluster
    version: 0.1.0
    repository: https://presslabs.github.io/charts
    condition: mysql.enabled
    alias: mysql

```

Once dependencies are configured run `helm dependency update` to fetch related charts.

More information about chart requirements can be found in the official [documentation](https://docs.helm.sh/developing_charts/#managing-dependencies-with-requirements-yaml).

### Configure your chart's values.yaml file

You can configure the cluster by providing values under the `mysql` key. A comprehensive description
can be found in the chart [`values.yaml`](https://github.com/presslabs/mysql-operator/blob/master/charts/mysql-cluster/values.yaml)
file.

```yaml
mysql:
  enabled: true
  rootPassword: <secure>
  appUser: <user name>
  appPassword: <user password>
  appDatabase: <app database>
```

### Use into your application

In your deployment add an environment variable that point to the `DB_CONNECT_URL` field from cluster secret named `{{ include "mysql-cluster.secretName" . }}`.

For example in the `deployment.yaml`:

```yaml
spec:
  replicas: {{ .Values.replicaCount }}
  template:
    spec:
      containers:
        - name: {{ .Chart.Name }}
          ...
          env:
            - name: DB_CONNECT_URL
              valueFrom:
                secretKeyRef:
                  name: {{ include "mysql-cluster.secretName" . }}
                  key: DB_CONNECT_URL
```

Now just modify your app to connect to the DSN that is provided into `DB_CONNECT_URL` environment variable.
