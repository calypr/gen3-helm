# drs-server Helm Chart

This chart deploys `drs-server` with:

- Config mounted into the pod at `/etc/drs/config.yaml` (not baked into the image)
- DB credentials injected via secret env vars (`DRS_DB_*`)
- Optional PostgreSQL init job that mirrors indexd-style setup:
  - creates app DB user
  - creates app database
  - applies DRS schema tables

## Key Compatibility Notes

- Secret keys mirror indexd credentials naming (`db_host`, `db_username`, `db_password`, `db_database`) with additional `db_port` and `db_sslmode`.
- In `gen3` mode, `drs-server` requires PostgreSQL.

## Install

```bash
helm upgrade --install drs-server ./helm/drs-server
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
