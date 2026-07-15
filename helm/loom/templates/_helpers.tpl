{{- define "loom.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "loom.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "loom.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
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
{{- printf "%s-clickstack-clickhouse-clickhouse-headless" .Release.Name -}}
{{- end -}}
{{- end -}}

{{- define "loom.clickhouseURL" -}}
{{- if .Values.server.clickhouse.url -}}
{{- .Values.server.clickhouse.url -}}
{{- else -}}
{{- printf "clickhouse://%s:%d" (include "loom.clickhouseHost" .) (int .Values.server.clickhouse.port) -}}
{{- end -}}
{{- end -}}
