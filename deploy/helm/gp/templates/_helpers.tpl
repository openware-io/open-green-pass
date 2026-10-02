{{- define "gp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "gp.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "gp.name" . -}}
{{- end -}}
{{- end -}}

{{- define "gp.labels" -}}
app.kubernetes.io/name: {{ include "gp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: greenpass
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
