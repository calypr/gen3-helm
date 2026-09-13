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

{{- define "syfon.deploymentName" -}}
syfon-deployment
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

{{- define "syfon.postgresTLSMode" -}}
{{- $global := .Values.global | default dict -}}
{{- $postgres := get $global "postgres" | default dict -}}
{{- $tls := get $postgres "tls" | default dict -}}
{{- $mode := get $tls "mode" | default "disabled" | toString | trim | lower -}}
{{- if not (has $mode (list "disabled" "selfsigned" "certmanager" "existingsecret")) -}}
  {{- fail (printf "global.postgres.tls.mode must be one of disabled, selfSigned, certManager, existingSecret; got %q" $mode) -}}
{{- end -}}
{{- $mode -}}
{{- end -}}

{{- define "syfon.postgresTLSSecretName" -}}
{{- $global := .Values.global | default dict -}}
{{- $postgres := get $global "postgres" | default dict -}}
{{- $tls := get $postgres "tls" | default dict -}}
{{- $mode := include "syfon.postgresTLSMode" . -}}
{{- $secretName := get $tls "secretName" | default "" | toString | trim -}}
{{- if ne $mode "disabled" -}}
  {{- if not $secretName -}}
    {{- fail "global.postgres.tls.secretName is required when PostgreSQL TLS is enabled" -}}
  {{- end -}}
{{- end -}}
{{- $secretName -}}
{{- end -}}

{{- define "syfon.postgresTLSCAKey" -}}
{{- $global := .Values.global | default dict -}}
{{- $postgres := get $global "postgres" | default dict -}}
{{- $tls := get $postgres "tls" | default dict -}}
{{- $mode := include "syfon.postgresTLSMode" . -}}
{{- $caKey := get $tls "caKey" | default "ca.crt" | toString | trim -}}
{{- if ne $mode "disabled" -}}
  {{- if not $caKey -}}
    {{- fail "global.postgres.tls.caKey is required when PostgreSQL TLS is enabled" -}}
  {{- end -}}
{{- end -}}
{{- $caKey -}}
{{- end -}}

{{- define "syfon.postgresTLSDatabaseSSLMode" -}}
{{- if eq (include "syfon.postgresTLSMode" .) "disabled" -}}disable{{- else -}}verify-full{{- end -}}
{{- end -}}

{{- define "syfon.postgresTLSAllowInsecureTransport" -}}
{{- if eq (include "syfon.postgresTLSMode" .) "disabled" -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{- define "syfon.postgresTLSRootCert" -}}
{{- if ne (include "syfon.postgresTLSMode" .) "disabled" -}}/etc/ssl/certs/postgres/ca.crt{{- end -}}
{{- end -}}

{{- define "syfon.validateLegacyPostgresTLSValues" -}}
{{- $postgres := .Values.postgres | default dict -}}
{{- $app := get $postgres "app" | default dict -}}
{{- $mode := include "syfon.postgresTLSDatabaseSSLMode" . -}}
{{- $allowInsecure := include "syfon.postgresTLSAllowInsecureTransport" . -}}
{{- if hasKey $app "db_sslmode" -}}
  {{- $configured := get $app "db_sslmode" | toString | trim | lower -}}
  {{- if ne $configured $mode -}}
    {{- fail (printf "postgres.app.db_sslmode=%q contradicts global.postgres.tls.mode=%q; omit the deprecated value or set it to %q" $configured (include "syfon.postgresTLSMode" .) $mode) -}}
  {{- end -}}
{{- end -}}
{{- if hasKey $app "allowInsecureTransport" -}}
  {{- $configured := get $app "allowInsecureTransport" | toString | trim | lower -}}
  {{- if ne $configured $allowInsecure -}}
    {{- fail (printf "postgres.app.allowInsecureTransport=%q contradicts global.postgres.tls.mode=%q; omit the deprecated value or set it to %s" $configured (include "syfon.postgresTLSMode" .) $allowInsecure) -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "syfon.validateExtraEnv" -}}
{{- $reserved := list "DRS_DB_SSLMODE" "DRS_DB_ALLOW_INSECURE_TRANSPORT" "PGSSLMODE" "PGSSLROOTCERT" -}}
{{- range $entry := .Values.extraEnv | default list -}}
  {{- $name := get $entry "name" | default "" | toString | trim -}}
  {{- if has $name $reserved -}}
    {{- fail (printf "extraEnv cannot override the database TLS variable %q" $name) -}}
  {{- end -}}
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
