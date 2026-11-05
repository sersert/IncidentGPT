{{- define "knowledge-mcp.fullname" -}}
{{- default .Release.Name .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "knowledge-mcp.labels" -}}
app.kubernetes.io/name: incidentgpt-knowledge-mcp
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: knowledge-mcp
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
