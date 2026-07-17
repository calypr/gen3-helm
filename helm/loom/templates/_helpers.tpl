{{- define "loom.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "loom.fullname" -}}
{{- default (include "loom.name" .) .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "loom.deploymentName" -}}
loom-deployment
{{- end }}

{{- define "loom.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/name: {{ include "loom.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "loom.selectorLabels" -}}
app.kubernetes.io/name: {{ include "loom.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "loom.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "loom.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "loom.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}

{{- define "loom.arangoURL" -}}
{{- if .Values.server.arango.url -}}
{{- .Values.server.arango.url -}}
{{- else -}}
{{- "http://arangodb:8529" -}}
{{- end -}}
{{- end -}}

{{- define "loom.clickhouseHost" -}}
{{- if .Values.server.clickhouse.host -}}
{{- .Values.server.clickhouse.host -}}
{{- else -}}
{{- "clickhouse" -}}
{{- end -}}
{{- end -}}

{{- define "loom.clickhouseURL" -}}
{{- if .Values.server.clickhouse.url -}}
{{- .Values.server.clickhouse.url -}}
{{- else -}}
{{- "clickhouse://clickhouse:9000" -}}
{{- end -}}
{{- end -}}
