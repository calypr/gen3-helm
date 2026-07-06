# funnel

Funnel is source-owned in `gen3-helm` and is wired into the Gen3 umbrella chart
as a local subchart through `file://../funnel`.

## Database

This chart is configured to use Gen3-managed PostgreSQL for Funnel task and
event storage. The previous bundled MongoDB dependency was removed because we
believe the MongoDB event writer is no longer used by this deployment path.

The Helm render verifies the generated Funnel config points at PostgreSQL and no
MongoDB resources are created. A deployed-cluster smoke test is still required
to prove the Funnel server binary successfully connects to and migrates/uses the
PostgreSQL database in a real environment.

Deployments that still need MongoDB event writing must reintroduce explicit
external MongoDB configuration.

## Cleanup CronJob

The optional cleanup CronJob runs:

```bash
funnel kubernetes cleanup --config /etc/config/funnel-server.yaml
```

It is intentionally disabled by default. Enable it with:

```yaml
cleanup:
  enabled: true
```

When `cleanup.schedule` is empty, the chart derives the CronJob schedule from
`Kubernetes.ReconcileRate`. Set `cleanup.schedule` to a standard 5-field cron
expression to override it.
