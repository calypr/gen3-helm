{{/*
Expand the name of the chart.
*/}}
{{- define "gen3.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "gen3.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "gen3.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gen3.labels" -}}
helm.sh/chart: {{ include "gen3.chart" . }}
{{ include "gen3.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gen3.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gen3.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "gen3.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "gen3.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
{{- define "gen3.postgresTLSMode" -}}
{{- $global := .Values.global | default dict -}}
{{- $postgres := get $global "postgres" | default dict -}}
{{- $tls := get $postgres "tls" | default dict -}}
{{- $mode := get $tls "mode" | default "disabled" | toString | trim | lower -}}
{{- if not (has $mode (list "disabled" "selfsigned" "certmanager" "existingsecret")) -}}
  {{- fail (printf "global.postgres.tls.mode must be one of disabled, selfSigned, certManager, existingSecret; got %q" $mode) -}}
{{- end -}}
{{- $mode -}}
{{- end -}}

{{- define "gen3.postgresTLSSecretName" -}}
{{- $global := .Values.global | default dict -}}
{{- $postgres := get $global "postgres" | default dict -}}
{{- $tls := get $postgres "tls" | default dict -}}
{{- $mode := include "gen3.postgresTLSMode" . -}}
{{- $secretName := get $tls "secretName" | default "" | toString | trim -}}
{{- if ne $mode "disabled" -}}
  {{- if not $secretName -}}
    {{- fail "global.postgres.tls.secretName is required when PostgreSQL TLS is enabled" -}}
  {{- end -}}
{{- end -}}
{{- $secretName -}}
{{- end -}}

{{- define "gen3.postgresTLSCAKey" -}}
{{- $global := .Values.global | default dict -}}
{{- $postgres := get $global "postgres" | default dict -}}
{{- $tls := get $postgres "tls" | default dict -}}
{{- $mode := include "gen3.postgresTLSMode" . -}}
{{- $caKey := get $tls "caKey" | default "ca.crt" | toString | trim -}}
{{- if ne $mode "disabled" -}}
  {{- if not $caKey -}}
    {{- fail "global.postgres.tls.caKey is required when PostgreSQL TLS is enabled" -}}
  {{- end -}}
{{- end -}}
{{- $caKey -}}
{{- end -}}
