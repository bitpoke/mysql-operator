---
title: Integration of MySQL Operator
linktitle: MySQL Operator Integration
description: MySQL Operator integration with your application
categories: [mysql operator]
keywords: [mysql operator]
menu:
  docs:
    parent: "mysqloperator"
weight: 3
draft: false
aliases: []
toc: true
---

After cluster creation, you can update the provided secret with a new field
named `DB_CONNECT_URL` that contains a DSN to connect to the writable cluster
endpoint. You can check the `_helper.tpl` file for more insight.

The MySQL operator provides three services to access the nodes:

 * `<cluster_name>-mysql-master` is the service that points to the master node
   and this endpoint should be used for writes. This service is usually used to
   construct the DSN.

 * `<cluster_name>-mysql` is the service that route traffic to all _healthy_
   nodes from the cluster. You should use this endpoint for reads.

 * `<cluster_name>-mysql-nodes` is the service used internally to access nodes. You
   can use this service to access a specific node(e.g.
   `<cluster_name>-mysql-0.<cluster-name>-mysql-nodes.default`)


We use helm to deploy our application into Kubernetes, so we updated our charts
to use Mysql operator to provide a cluster per application.

#### Helm chart
In the next paragraphs are show some parts of a helm chart, only the parts that
provide a MySQL Cluster. You can find a more comprehensive guide to writing a
helm chart [here](https://docs.helm.sh/developing_charts/).

In this section, we'll exemplify parts from a chart that can be used to
integrate the MySQL Operator with an application using Helm. The chart structure
is as follows:

```
chart
├── Chart.yaml
├── templates
│   ├── deployment.yaml
│   ├── mysql-backup-secret.yaml
│   ├── mysql-secret.yaml
│   ├── mysql.yaml
|   └── _helpers.tpl
└── values.yaml
```

#### The `values.yaml` file

The `values.yaml` contains chart defaults values.

```yaml
mysql:
  # if the MySQL is enabled or not, this can be put under a flag
  enabled: true
  # how many MySQL cluster nodes
  replicas: 1
  # the application user to be created
  dbUser: app
  # the application database name to be created
  dbDatabase: application
  # the application password
  dbPassword: password
  # the root password
  rootPassword: <random>
  # some extra specification

  # backup scheduler for reccurent backups
  backupSchedule:
  # backup URL to put backups
  backupURL:
  # a secret that contains storage credentails
  backupSecretName:
  # or storage credentails, the backup secret content
  backupCredentials:
    # GCS_SERVICE_ACCOUNT_JSON_KEY: ?
    # GCS_PROJECT_ID: ?
```

#### The `mysql.yaml` file

This file contains the Mysql cluster resource as a go template.

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: {{ .Release.Name}}-db
spec:
  replicas: {{ .Values.mysql.replicas }}
  secretName: {{ .Release.Name }}-db-secret

  {{- if .Values.mysql.backupSecretName }}
  backupSecretName: {{ .Values.mysql.backupSecretName }}
  {{- else if .Values.mysql.backupCredentials }}
  backupSecretName: {{ .Release.Name }}-backup-secret
  {{- else if .Values.mysql.backupSchedule }}
  {{ required "One of .mysql.backupBucketSecretName and .mysql.backupCredentails should be specified" "" }}
  {{- end }}

  {{- if .Values.mysql.backupSchedule }}
  backupSchedule: {{ .Values.mysql.backupSchedule }}
  backupURL: {{ required ".mysql.backupBucketURL is missing" .Values.mysql.backupURL }}
  {{- end }}
```


#### The `mysql-secret.yaml` file

This file describes how to create the secret that contains all needed fields.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-db-secret
type: Opaque
data:
  ROOT_PASSWORD: {{ required ".mysql.rootPassword is missing" .Values.mysql.rootPassword | b64enc | quote }}
  USER: {{ required ".mysql.dbUser is missing" .Values.mysql.dbUser | b64enc | quote }}
  PASSWORD: {{ required ".mysql.dbPassword is missing" .Values.mysql.dbPassword | b64enc | quote }}
  DATABASE: {{ required ".mysql.dbDatabase is missing" .Values.mysql.dbDatabase | b64enc | quote }}
  DB_CONNECT_URL: {{ include "app.mysql_connect_url" . | b64enc | quote }}
```


#### The `mysql-backup-secret.yaml` file

This file describes how to create the secret that contains credentials for the
storage provider.

```yaml
{{- if .Values.mysql.backupCredentials }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-backup-secret
  labels:
    app: {{ template "name" . }}
    chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
    release: {{ .Release.Name }}
    heritage: {{ .Release.Service }}
type: Opaque
data:
    {{- range $key, $value := .Values.mysql.backupCredentials }}
    {{ $key | upper }}: {{ $value | b64enc | quote }}
    {{ end }}
{{- end }}
```


#### The example `deployment.yaml` file

This file is an example of how to link the `DB_CONNECT_URL` from the DB secret
to the application environment.

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-web
spec:
  replicas: {{ .Values.web.replicaCount }}
  template:
    metadata:
      labels:
        app: {{ template "name" . }}
        release: {{ .Release.Name }}
        role: web
    spec:
      containers:
        - name: {{ .Release.Name }}-web
          image: "{{ .Values.image }}"
          imagePullPolicy: {{ .Values.imagePullPolicy }}
          readinessProbe:
          env:
          - name: {{ printf "%s%s" $envPrefix "DB_CONNECT_URL" }}
            valueFrom:
              secretKeyRef:
                name: {{ .Release.Name }}-db-secret
                key: DB_CONNECT_URL
```


#### The helper `_helpers.tpl` file

By convention, we put all helper templates in this file. Here is the template
that builds `DB_CONNECT_URL`. The DSN is constructed using the master service,
that points to the master nodes, so we can read and write to this DSN.

```yaml
{{- define "app.mysql_connect_url" -}}
mysql://{{- .Values.mysql.dbUser -}}:{{- .Values.mysql.dbPassword -}}@
{{- include "app.name_mysql" . -}}-mysql-master:3306/{{- .Values.mysql.dbDatabase -}}
{{- end -}}
```
