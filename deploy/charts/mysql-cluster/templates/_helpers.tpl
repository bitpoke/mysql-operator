{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "mysql-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "mysql-cluster.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "mysql-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mysql-cluster.dbConnectURL" -}}
mysql://
{{- if .Values.appUser -}}
    {{ urlquery .Values.appUser -}}
    {{- if .Values.appPassword -}}
        :{{ urlquery .Values.appPassword }}
    {{- end -}}
@
{{- end -}}
{{- include "mysql-cluster.clusterName" . -}}-mysql-master:3306
{{- if .Values.appDatabase -}}
/{{- .Values.appDatabase -}}
{{- end -}}
{{- end -}}

{{- define "mysql-cluster.clusterName" -}}
{{- printf "%s-db" (include "mysql-cluster.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mysql-cluster.secretName" -}}
{{- printf "%s-db" (include "mysql-cluster.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mysql-cluster.backupSecretName" -}}
{{- printf "%s-db-backup" (include "mysql-cluster.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
