{{- define "incidentgpt-sanitizer.name" -}}
{{- .Chart.Name -}}
{{- end }}

{{- define "incidentgpt-sanitizer.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Chart.Name .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end }}

{{- define "incidentgpt-sanitizer.labels" -}}
app.kubernetes.io/name: {{ include "incidentgpt-sanitizer.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: sanitizer
{{- end }}

{{- define "incidentgpt-sanitizer.secretName" -}}
{{- default (printf "%s-secrets" (include "incidentgpt-sanitizer.fullname" .)) .Values.secrets.existingSecret -}}
{{- end }}
