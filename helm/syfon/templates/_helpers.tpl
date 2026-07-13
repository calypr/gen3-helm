{{- define "syfon.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "syfon.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "syfon.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "syfon.labels" -}}
app.kubernetes.io/name: {{ include "syfon.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "syfon.selectorLabels" -}}
app.kubernetes.io/name: {{ include "syfon.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "syfon.appDbSecretName" -}}
{{- if .Values.postgres.app.existingSecret -}}
{{- .Values.postgres.app.existingSecret -}}
{{- else -}}
{{- .Values.postgres.app.secretName -}}
{{- end -}}
{{- end -}}

{{- define "syfon.adminDbSecretName" -}}
{{- if .Values.postgres.admin.existingSecret -}}
{{- .Values.postgres.admin.existingSecret -}}
{{- else -}}
{{- .Values.postgres.admin.secretName -}}
{{- end -}}
{{- end -}}

{{- define "syfon.fenceURL" -}}
{{- $cfg := .Values.config | default dict -}}
{{- $auth := get $cfg "auth" | default dict -}}
{{- $configured := get $auth "fence_url" | default "" | toString | trim -}}
{{- if $configured -}}
{{- $configured -}}
{{- else -}}
{{- $global := .Values.global | default dict -}}
{{- $hostname := get $global "hostname" | default "" | toString | trim -}}
{{- if $hostname -}}
{{- $host := trimSuffix "/" (trimPrefix "https://" (trimPrefix "http://" $hostname)) -}}
{{- printf "https://%s/user" $host -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Generate or reuse a secret value for compatibility credentials.
*/}}
{{- define "syfon.getOrGenSecret" -}}
{{- $value := index . 0 -}}
{{- $secretName := index . 1 -}}
{{- $secretKey := index . 2 -}}
{{- $secretLength := index . 3 -}}
{{- $namespace := index . 4 -}}
{{- if $value -}}
{{- $value = $value | b64enc -}}
{{- end -}}
{{- if not $value -}}
  {{- if $secret := lookup "v1" "Secret" $namespace $secretName -}}
    {{- if hasKey $secret.data $secretKey -}}
      {{- $value = index $secret.data $secretKey -}}
    {{- end -}}
  {{- end -}}
  {{- if not $value -}}
    {{- $value = randAlphaNum $secretLength | b64enc -}}
  {{- end -}}
{{- end -}}
{{- $value -}}
{{- end -}}
