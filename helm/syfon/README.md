# syfon Helm Chart

This chart deploys `syfon` with:

- Syfon config mounted into the pod at `/etc/drs/config.yaml` from `config`
  using a Kubernetes Secret
- DB credentials injected via secret env vars (`DRS_DB_*`)
- Optional PostgreSQL init job that mirrors indexd-style setup:
  - creates app DB user
  - creates app database
  - applies DRS schema tables

## Syfon Config

`config` is rendered directly as Syfon's server config. Use the same keys that
`syfon serve --config` accepts.

Example:

```yaml
config:
  port: 8080
  auth:
    mode: gen3
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
  s3_credentials:
    - bucket: cbds
      provider: s3
      region: us-east-1
      endpoint: https://s3.example.org
      access_key: access-key
      secret_key: secret-key
      resources:
        - organization: cbds
          project: training
          org_path: programs/cbds
          project_path: projects/training
  bucket_scopes:
    - organization: cbds
      project_id: training
      bucket: cbds
      org_path: programs/cbds
      project_path: projects/training
```

Configured `s3_credentials` are loaded by the Syfon server on startup. Syfon
requires a credential encryption key for non-empty bucket credentials; set
`credential_encryption.master_key` in the same config block. The key may be a
32-byte raw string, a 64-character hex string, or base64-encoded 32-byte key.

Configured `bucket_scopes` are loaded on startup too. `organization` and
`project_id` are the Gen3 authz labels. `organization_sub_path` /
`project_sub_path` are storage-layout prefixes, and the chart also accepts the
shorter aliases `org_path` / `project_path` and normalizes them in the rendered
config. You can define these scopes either as top-level `bucket_scopes` or
inline under each `s3_credentials[*].resources` entry. Inline resource entries
use `organization`, `project`, `org_path`, and `project_path`; the chart
attaches the parent bucket and renders the final server config as normal
`bucket_scopes`. Syfon stores the full scope prefix and prepends that prefix
when signing imported record URLs that are relative to the bucket root. You can
also set a complete `path` or explicit `bucket` plus `path_prefix`.

## Key Compatibility Notes

- Secret keys mirror indexd credentials naming (`db_host`, `db_username`, `db_password`, `db_database`) with additional `db_port` and `db_sslmode`.
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
