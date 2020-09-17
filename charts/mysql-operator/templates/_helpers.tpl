{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "mysql-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "mysql-operator.fullname" -}}
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
{{- define "mysql-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "mysql-operator.serviceAccountName" -}}
{{- if .Values.rbac.create -}}
    {{ include "mysql-operator.fullname" . }}
{{- else -}}
    {{ default "default" .Values.rbac.serviceAccountName }}
{{- end -}}
{{- end -}}

{{- define "mysql-operator.raftlist" -}}
{{- $fullname := include "mysql-operator.fullname" . -}}
{{- $replicas := int .Values.replicas -}}
{{- range $i := until $replicas -}}
{{ $fullname }}-{{ $i }}-svc{{- if lt $i (sub $replicas 1) }},{{ end }}
{{- end -}}
{{- end -}}

{{- define "mysql-operator.orc-config-name" -}}
{{ include "mysql-operator.fullname" . }}-orc
{{- end -}}

{{- define "mysql-operator.orc-secret-name" -}}
{{- if .Values.orchestrator.secretName -}}
  {{ .Values.orchestrator.secretName }}
{{- else -}}
  {{ include "mysql-operator.fullname" . }}-orc
{{- end -}}
{{- end -}}

{{- define "mysql-operator.orc-service-name" -}}
{{ include "mysql-operator.fullname" . }}-orc
{{- end -}}

{{/*
Common labels
*/}}
{{- define "mysql-operator.labels" -}}
app.kubernetes.io/name: {{ include "mysql-operator.name" . }}
helm.sh/chart: {{ include "mysql-operator.chart" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}