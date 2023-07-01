{{- define "orchestrator.name" -}}
{{- $name := include "mysql-operator.name" . | trunc 50 | trimSuffix "-" }}
{{- printf "%s-orchestrator" $name  }}
{{- end }}

{{- define "orchestrator.fullname" -}}
{{- $fullname := include "mysql-operator.fullname" . | trunc 59 | trimSuffix "-" }}
{{- printf "%s-orc" $fullname  }}
{{- end }}

{{- define "orchestrator.raftList" -}}
{{- $replicas := int .Values.replicaCount }}
{{- $fullname := include "mysql-operator.fullname" . }}
{{- $nodes := (dict)  }}
{{- range $i := until $replicas }}
{{- $_ := set $nodes (printf "%d" $i) (printf "%s-%d-orc-svc" $fullname $i) }}
{{- end }}
{{- values $nodes | sortAlpha | join "," }}
{{- end }}

{{- define "orchestrator.secretName" -}}
{{- if .Values.orchestrator.secretName -}}
{{ .Values.orchestrator.secretName }}
{{- else -}}
{{ include "orchestrator.fullname" . }}
{{- end -}}
{{- end }}

{{- define "orchestrator.apiURL" -}}
{{- $port := "" }}
{{- if ne (printf "%d" .Values.orchestrator.service.port) "80" }}
{{- $port := printf ":$d" .Values.orchestrator.service.port }}
{{- end -}}
http://{{ template "mysql-operator.fullname" . }}.{{ .Release.Namespace }}{{ $port }}/api
{{- end }}
