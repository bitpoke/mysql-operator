{{- define "name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}


{{- define "hlService" -}}
{{ template "fullname" . }}-orc
{{- end -}}


{{- define "topology-cnf" -}}
[client]
user = orchestrator
password = {{ randAlphaNum 18 }}

{{- end -}}
