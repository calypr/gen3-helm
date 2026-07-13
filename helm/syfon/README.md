# syfon Helm Chart

This chart deploys `syfon` with:

- Syfon config mounted into the pod at `/etc/drs/config.yaml` from `config`
  using a Kubernetes Secret
- DB credentials injected via secret env vars (`DRS_DB_*`)
- A compatibility service-creds secret for legacy Fence/Sheepdog consumers
- Optional PostgreSQL init job that mirrors indexd-style setup:
  - creates app DB user
  - creates app database
  - applies DRS schema tables

## Syfon Config

`config` is rendered directly as Syfon's server config. Use the same keys that
`syfon serve --config` accepts.

In `gen3` auth mode, the chart fills `config.auth.fence_url` from
`global.hostname` when it is omitted, rendering it as
`https://<global.hostname>/user`. Set `config.auth.fence_url` explicitly only
when Syfon should trust a different public Fence endpoint.

Example:

```yaml
config:
  port: 8080
  auth:
    mode: gen3
    # Optional; defaults to https://<global.hostname>/user
    fence_url: https://gen3.example.org/user
  routes:
    docs: true
    ga4gh: true
    internal: true
    lfs: true
    metrics: true
  signing:
    default_expiry_seconds: 900
  credential_encryption:
    master_key: base64-or-hex-or-32-byte-key
  buckets:
    - bucket: cbds
      provider: s3
      region: us-east-1
      endpoint: https://s3.example.org
      access_key: access-key
      secret_key: secret-key
      resources:
        - organization: cbds
          org_path: programs/cbds
          projects:
            - project_id: training
              project_path: projects/training
  bucket_scopes:
    - organization: cbds
      project_id: training
      bucket: cbds
      path_prefix: programs/cbds/projects/training
```

Legacy bucket-keyed config remains valid:

```yaml
config:
  s3_credentials:
    - bucket: cbds
      provider: s3
      region: us-east-1
      access_key: access-key
      secret_key: secret-key
  bucket_scopes:
    - organization: cbds
      project_id: training
      bucket: cbds
```

Configured `buckets` are loaded by the Syfon server on startup. Operators use
the physical `bucket` name in config and API requests. Syfon handles any
internal database keying itself from the non-secret credential fields, and
`secret_key` rotation does not change that internal key.

Syfon requires a credential encryption key for non-empty bucket credentials; set
`credential_encryption.master_key` in the same config block. The key may be a
32-byte raw string, a 64-character hex string, or base64-encoded 32-byte key.

Configured `bucket_scopes` are loaded on startup too. `organization` and
`project_id` are the Gen3 authz labels. Scopes reference the physical `bucket`
and set `path_prefix` when the project should write below a specific key prefix.
`organization_sub_path` / `project_sub_path` are storage-layout prefixes, and
inline `buckets[*].resources` entries can still derive normal `bucket_scopes`.
If the same physical bucket is configured more than once with different
credentials, define scopes inline under the intended `buckets[*].resources`
entry so the scope is unambiguous without exposing internal identity fields.

Migration rule: leave existing `bucket` references in place. Do not add any
extra credential identifier values to values.yaml.

## Legacy Credential Cleanup

Older Syfon deployments used the physical bucket name as the database
`s3_credential.credential_id`. Current Syfon derives a stable internal
credential ID from non-secret credential fields. After switching to the derived
ID model, a database can temporarily contain both rows:

- legacy row: `credential_id = bucket`
- current row: derived credential ID for the same physical bucket

If the legacy row was encrypted with an old `credential_encryption.master_key`,
`GET /data/buckets` will fail while listing credentials. Use the chart helper
script to clean this up.

Dry-run first:

```bash
KUBECTL=kc ./helm/syfon/scripts/cleanup-legacy-credential-ids.sh \
  --namespace default \
  --db-host local-postgresql \
  --db-name syfon_db \
  --secret syfon-db-admin
```

Apply:

```bash
KUBECTL=kc ./helm/syfon/scripts/cleanup-legacy-credential-ids.sh \
  --namespace default \
  --db-host local-postgresql \
  --db-name syfon_db \
  --secret syfon-db-admin \
  --apply
```

The script is intentionally conservative. It only remaps `bucket_scope` rows and
deletes legacy credential rows when exactly one replacement credential exists
for the same physical bucket. Ambiguous buckets are skipped and must be reviewed
manually.

## Key Compatibility Notes

- Secret keys mirror indexd credentials naming (`db_host`, `db_username`, `db_password`, `db_database`) with additional `db_port` and `db_sslmode`.
- By default the chart also creates `indexd-service-creds` so existing Fence and
  Sheepdog deployments can keep reading `fence` / `sheepdog` service passwords
  after migrating the backend service to Syfon.
- In `gen3` mode, `syfon` requires PostgreSQL.
- The rendered Syfon config is stored as a Kubernetes Secret because it can
  contain bucket credentials and `credential_encryption.master_key`.
- Database connection values are still supplied from Kubernetes secrets via
  `DRS_DB_*` env vars.

## Install

```bash
helm upgrade --install syfon ./helm/syfon
```

## Existing Secrets

To reuse existing DB secrets:

- Set `postgres.app.existingSecret`
- Set `postgres.admin.existingSecret` (if `postgres.initJob.enabled=true`)

## Health Probes

The chart configures both readiness and liveness probes against `GET /healthz` on the container `http` port.

Tune probe behavior via:

- `probes.liveness.*`
- `probes.readiness.*`

## PostgreSQL Source of Truth

By default this chart now inherits PostgreSQL host/port/admin credentials from `global.postgres.master.*` (the same pattern used by other Gen3 charts).

Service-specific values under `postgres.app.*` and `postgres.admin.*` still override global values when set.
