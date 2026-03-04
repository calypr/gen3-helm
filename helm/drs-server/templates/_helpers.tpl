{{- define "drs-server.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "drs-server.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "drs-server.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "drs-server.labels" -}}
app.kubernetes.io/name: {{ include "drs-server.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "drs-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "drs-server.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "drs-server.appDbSecretName" -}}
{{- if .Values.postgres.app.existingSecret -}}
{{- .Values.postgres.app.existingSecret -}}
{{- else -}}
{{- .Values.postgres.app.secretName -}}
{{- end -}}
{{- end -}}

{{- define "drs-server.adminDbSecretName" -}}
{{- if .Values.postgres.admin.existingSecret -}}
{{- .Values.postgres.admin.existingSecret -}}
{{- else -}}
{{- .Values.postgres.admin.secretName -}}
{{- end -}}
{{- end -}}

