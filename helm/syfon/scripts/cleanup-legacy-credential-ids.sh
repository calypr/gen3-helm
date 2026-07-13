#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
cleanup-legacy-credential-ids.sh

Safely remap Syfon bucket scopes away from legacy bucket-name credential IDs and
delete stale legacy credential rows.

Default mode is a dry-run. Use --apply to mutate the database.

Options:
  --namespace <ns>        Kubernetes namespace. Default: default
  --db-host <host>        PostgreSQL service host. Default: local-postgresql
  --db-name <db>          Database name. Default: syfon_db
  --db-user <user>        Database user. Default: postgres
  --secret <name>         Kubernetes secret with DB password. Default: syfon-db-admin
  --secret-key <key>      Secret key to read. Default: auto-detect password/db_password
  --image <image>         psql client image. Default: postgres:16
  --kubectl <binary>      kubectl-compatible binary. Default: kubectl, then kc
  --apply                 Apply changes. Without this, only prints candidates.
  -h, --help              Show help.

Example:
  KUBECTL=kc ./cleanup-legacy-credential-ids.sh --namespace default --apply

This script only remaps/deletes legacy rows when exactly one replacement
credential exists for the same physical bucket. Ambiguous buckets are skipped.
USAGE
}

namespace="default"
db_host="local-postgresql"
db_name="syfon_db"
db_user="postgres"
secret_name="syfon-db-admin"
secret_key=""
image="postgres:16"
kubectl_bin="${KUBECTL:-}"
apply="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      namespace="$2"
      shift 2
      ;;
    --db-host)
      db_host="$2"
      shift 2
      ;;
    --db-name)
      db_name="$2"
      shift 2
      ;;
    --db-user)
      db_user="$2"
      shift 2
      ;;
    --secret)
      secret_name="$2"
      shift 2
      ;;
    --secret-key)
      secret_key="$2"
      shift 2
      ;;
    --image)
      image="$2"
      shift 2
      ;;
    --kubectl)
      kubectl_bin="$2"
      shift 2
      ;;
    --apply)
      apply="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$kubectl_bin" ]]; then
  if command -v kubectl >/dev/null 2>&1; then
    kubectl_bin="kubectl"
  elif command -v kc >/dev/null 2>&1; then
    kubectl_bin="kc"
  else
    echo "kubectl-compatible binary not found; set KUBECTL=kc or pass --kubectl" >&2
    exit 1
  fi
fi

read_secret_key() {
  local key="$1"
  local encoded
  encoded="$("$kubectl_bin" get secret "$secret_name" -n "$namespace" -o "jsonpath={.data.${key}}" 2>/dev/null || true)"
  if [[ -n "$encoded" ]]; then
    printf '%s' "$encoded" | base64 -d
    return 0
  fi
  return 1
}

if [[ -n "$secret_key" ]]; then
  if ! pgpassword="$(read_secret_key "$secret_key")"; then
    echo "secret $secret_name does not contain key $secret_key" >&2
    exit 1
  fi
else
  if pgpassword="$(read_secret_key "password")"; then
    :
  elif pgpassword="$(read_secret_key "db_password")"; then
    :
  else
    echo "secret $secret_name does not contain password or db_password" >&2
    exit 1
  fi
fi

pod_name="syfon-psql-debug-$(date +%s)"

run_psql() {
  "$kubectl_bin" run "$pod_name" \
    -n "$namespace" \
    --rm -i \
    --restart=Never \
    --image="$image" \
    --env "PGPASSWORD=$pgpassword" \
    -- psql -h "$db_host" -U "$db_user" -d "$db_name" -v ON_ERROR_STOP=1
}

dry_run_sql='
\pset pager off
\echo Safe legacy credential replacements:
WITH legacy AS (
  SELECT credential_id AS old_id, bucket
  FROM s3_credential
  WHERE credential_id = bucket
),
replacement_counts AS (
  SELECT l.old_id, l.bucket, count(d.credential_id) AS replacement_count, max(d.credential_id) AS new_id
  FROM legacy l
  JOIN s3_credential d ON d.bucket = l.bucket AND d.credential_id <> l.old_id
  GROUP BY l.old_id, l.bucket
),
safe_replacements AS (
  SELECT old_id, new_id, bucket
  FROM replacement_counts
  WHERE replacement_count = 1
)
SELECT old_id, new_id, bucket,
       (SELECT count(*) FROM bucket_scope s WHERE s.credential_id = safe_replacements.old_id) AS scopes_to_remap
FROM safe_replacements
ORDER BY bucket, old_id;

\echo Ambiguous legacy credentials skipped:
WITH legacy AS (
  SELECT credential_id AS old_id, bucket
  FROM s3_credential
  WHERE credential_id = bucket
),
replacement_counts AS (
  SELECT l.old_id, l.bucket, count(d.credential_id) AS replacement_count
  FROM legacy l
  LEFT JOIN s3_credential d ON d.bucket = l.bucket AND d.credential_id <> l.old_id
  GROUP BY l.old_id, l.bucket
)
SELECT old_id, bucket, replacement_count
FROM replacement_counts
WHERE replacement_count <> 1
ORDER BY bucket, old_id;

\echo Existing dangling bucket scopes:
SELECT s.organization, s.project_id, s.credential_id, s.bucket
FROM bucket_scope s
LEFT JOIN s3_credential c ON c.credential_id = s.credential_id
WHERE c.credential_id IS NULL
ORDER BY s.organization, s.project_id, s.credential_id;
'

apply_sql='
\pset pager off
BEGIN;

CREATE TEMP TABLE syfon_legacy_credential_replacements AS
WITH legacy AS (
  SELECT credential_id AS old_id, bucket
  FROM s3_credential
  WHERE credential_id = bucket
),
replacement_counts AS (
  SELECT l.old_id, l.bucket, count(d.credential_id) AS replacement_count, max(d.credential_id) AS new_id
  FROM legacy l
  JOIN s3_credential d ON d.bucket = l.bucket AND d.credential_id <> l.old_id
  GROUP BY l.old_id, l.bucket
)
SELECT old_id, new_id, bucket
FROM replacement_counts
WHERE replacement_count = 1;

\echo Remapping bucket scopes:
UPDATE bucket_scope s
SET credential_id = r.new_id
FROM syfon_legacy_credential_replacements r
WHERE s.credential_id = r.old_id;

\echo Deleting stale legacy credential rows:
DELETE FROM s3_credential c
USING syfon_legacy_credential_replacements r
WHERE c.credential_id = r.old_id;

COMMIT;

\echo Remaining dangling bucket scopes:
SELECT s.organization, s.project_id, s.credential_id, s.bucket
FROM bucket_scope s
LEFT JOIN s3_credential c ON c.credential_id = s.credential_id
WHERE c.credential_id IS NULL
ORDER BY s.organization, s.project_id, s.credential_id;

\echo Remaining legacy credentials with derived replacements:
WITH legacy AS (
  SELECT credential_id AS old_id, bucket
  FROM s3_credential
  WHERE credential_id = bucket
)
SELECT l.old_id, l.bucket, count(d.credential_id) AS replacement_count
FROM legacy l
JOIN s3_credential d ON d.bucket = l.bucket AND d.credential_id <> l.old_id
GROUP BY l.old_id, l.bucket
ORDER BY l.bucket, l.old_id;
'

if [[ "$apply" == "true" ]]; then
  echo "Applying Syfon legacy credential cleanup in namespace $namespace against $db_host/$db_name"
  printf '%s\n' "$apply_sql" | run_psql
else
  echo "Dry-run Syfon legacy credential cleanup in namespace $namespace against $db_host/$db_name"
  echo "Pass --apply to perform the remap/delete transaction."
  printf '%s\n' "$dry_run_sql" | run_psql
fi
