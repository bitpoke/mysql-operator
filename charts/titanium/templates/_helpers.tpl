{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "serviceName" -}}
{{ .Release.Name }}-mysql
{{- end -}}

{{- define "readServiceName" -}}
{{- include "serviceName" . -}}-read
{{- end -}}

{{- define "master-node-name" -}}
{{ template "fullname" . }}-0
{{- end -}}

{{- define "dbUser" -}}
{{ required "Missing mysql.dbUser" .Values.mysql.dbUser }}
{{- end -}}

{{- define "dbPassword" -}}
{{ required "Missing .mysql.dbPassword" .Values.mysql.dbPassword }}
{{- end -}}

{{- define "dbName" -}}
{{ required "Missing .mysql.dbName" .Values.mysql.dbName }}
{{- end -}}

{{- define "dbHost" -}}
{{- template "master-node-name" . }}.{{ template "serviceName" . }}
{{- end -}}

{{- define "dbPort" -}}
3306
{{- end -}}

{{- define "db-connect-url" -}}
mysql://{{ template "dbUser" . }}:{{ template "dbPassword" . }}@
{{- template "dbHost" . }}:{{ template "dbPort" }}/
{{- template "dbName" . -}}?charset=utf8mb4&conn_max_age=300
{{- end -}}

{{- define "mysqld-cnf" -}}
default-storage-engine         = InnoDB
gtid-mode                      = on
enforce-gtid-consistency       = on

{{- range $key, $value := .Values.mysqlConfig }}
{{ $key }} = {{ $value }}
{{- end }}

{{- end -}}

{{- define "titanium.volumeMount" }}
- name: data
  mountPath: /var/lib/mysql
  subPath: mysql
- name: conf
  mountPath: /etc/mysql
- name: secret-conf  # those are used for rclone
  mountPath: /var/run/secrets/
- name: config-map
  mountPath: /mnt/config-map
{{ end -}}

{{- define "titanium.env.rootPassword" }}
{{- if .Values.mysql.allowEmptyPassword }}
- name: MYSQL_ALLOW_EMPTY_PASSWORD
  value: "true"
{{- else }}
- name: MYSQL_ROOT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-mysql-secrets
      key: MYSQL_ROOT_PASSWORD
{{- end }}
{{ end -}}

{{- define "titanium.env.replication" }}
- name: TITANIUM_REPLICATION_USER
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-mysql-secrets
      key: TITANIUM_REPLICATION_USER
- name: TITANIUM_REPLICATION_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ .Release.Name }}-mysql-secrets
      key: TITANIUM_REPLICATION_PASSWORD
{{ end -}}

{{- define "titanium.env.rootPassword" }}
{{ end -}}


{{- define "titanium.env" }}
- name: TITANIUM_BACKUP_BUCKET
  value: {{ .Values.backupBucket }}
{{- if .Values.backupPrefix }}
- name: TITANIUM_BACKUP_PREFIX
  value: {{ .Values.backupPrefix }}
{{- end }}
- name: TITANIUM_RELEASE_NAME
  value: {{ template "fullname" . }}
- name: TITANIUM_GOVERNING_SERVICE
  value: {{ template "serviceName" . }}
- name: TITANIUM_INIT_BUCKET_URI
  value: {{ .Values.initBucketURI }}
{{ end -}}