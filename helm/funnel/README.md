# Funnel

This chart deploys Funnel v0.12.3 with the Gen3 authorization plugin. The Gen3 umbrella chart keeps Funnel disabled by default. Set `funnel.enabled: true` to enable the publication service.

## Deployment requirements

Set these values before enabling Funnel:

- `Server.OidcAuth.ServiceConfigURL`, `ClientId`, `ClientSecret`, `RequireAudience`, and `RedirectURL`. The chart rejects empty values. Set `RedirectURL` to the public Funnel login URL, such as `https://commons.example.org/funnel/login`.
- `storage.workspace.storageClassName` to the OpenStack CSI StorageClass. The chart creates a native, per-task workspace claim with `ReadWriteOnce` access and a default size of `10Gi`.
- `Plugins.Params.ETL_IMAGE` to the operator-approved metadata ETL image. The chart has no default ETL image.
- `Plugins.Params.DRS_URL` to the trusted DRS/Syfon service. The default is `http://syfon:8080`.
- `Plugins.Params.LOOM_URL` if Loom is not reachable at `http://loom:8080`.

Funnel uses the Gen3 chart's PostgreSQL server and provisions a dedicated `funnel` database role. Set the existing PostgreSQL connection values for the environment. The server config includes OIDC and database credentials in a ConfigMap. Restrict ConfigMap read access to trusted operators and Funnel.

The chart does not enable Loom. Deploy a compatible Loom service separately and set `loom.enabled: true` in the umbrella values when the metadata publication flow is ready for that Loom API. The publication path remains opt-in. The Loom image must include `POST /api/v1/datasets/{project}/sync-and-publish`; an older image with only generation uploads will not work. The dedicated `funnel-job` worker image builds from `calypr_etl_pod` using its pinned Loom client module and runs `/app/funnel-etl`.

## Worker storage and credentials

Funnel creates one native workspace claim per task. The worker and executor remain separate Jobs. The executor has required pod affinity to the task's worker, so both Pods can mount an RWO claim on the same node. The chart creates no task PersistentVolumes and does not configure S3 storage.

The plugin writes task-specific worker config to a ConfigMap. The config contains the caller access token, input data, and PostgreSQL credentials. Restrict ConfigMap read access to trusted operators and Funnel.

The authorizer image workflow builds `components/funnel-gen3-auth/Dockerfile` and publishes `ghcr.io/calypr/funnel-gen3-auth`. Set the chart's plugin image tag to the built commit tag. If the package is private, set `imagePullSecrets` for the Funnel server and task Jobs.

## Selected-file publication

The browser submits canonical `META/<FHIRResourceType>.ndjson` paths at a Git commit. The ETL uses the existing Git-DRS library to fetch those files and streams them to Loom. Loom computes content checksums and uses the Funnel task ID to recognize retries. Git-DRS itself is unchanged.

Each selected file replaces its previous records, outgoing relationships, and catalog entries. An empty file clears that file's previous records. Unselected files remain intact. Projects with an active immutable dataset generation, or existing untracked records for a selected type, require an explicit migration before using this path.

Graph replacement and ClickHouse publication are separate commits. If publication fails, the previous ClickHouse output remains available while the recorded operation can be retried. A successful task requires Loom's published receipt. Tokens may expire while a job waits in the queue; this implementation does not refresh them.

The worker image contains the import executable and CA certificates. It performs GitHub source acquisition and Git-DRS hydration, then submits load/publication requests using Loom's standalone HTTP client. Loom's server image has no Git-DRS dependency or job executable.
