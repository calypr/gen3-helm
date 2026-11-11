# Funnel Gen3 publish plugin

This executable replaces Sower's Gen3 permission check for Funnel metadata publish tasks. It uses Funnel's plugin authentication API.

## Configuration

The Funnel chart must set these `Plugins.Params` values from trusted deployment configuration:

| Parameter | Purpose |
| --- | --- |
| `FENCE_URL` | Fence service base URL for caller verification and GitHub credential exchange. |
| `GECKO_URL` | Gecko service base URL for registered project configuration. |
| `DRS_URL` | Trusted DRS/Syfon base URL used by the worker, such as `http://syfon:8080`. |
| `LOOM_URL` | Trusted internal Loom URL used by the ETL worker. |
| `ETL_IMAGE` | Operator selected image containing `/app/funnel-etl`. |

The `Kubernetes.ExecutorTemplate` must contain exactly one `# CALYPR_ETL_ENV` marker on its executor container `env:` list. The plugin replaces it with per-task `ACCESS_TOKEN`, `GITHUB_TOKEN`, `INPUT_DATA`, and trusted `LOOM_URL`, `DRS_URL`, and `ETL_WORKSPACE_DIR` values. It returns a full cloned Funnel configuration for per-task execution. The caller's credentials do not enter the replacement TES task or its tags.

The task image and command come from trusted `ETL_IMAGE` and the fixed `/app/funnel-etl` command. The task uses the fixed `/workspace` working directory and volume. A publish request must use the `calypr.publish` tag with `organization`, `project`, `revision`, and a nonempty list of direct `META/*.ndjson` paths. The plugin rejects tasks that supply executors, inputs, outputs, resources, volumes, or `_WORKER_SA`.

For `CREATE`, the plugin forwards the caller's bearer authorization to Fence `/user/`, checks the same parent resources and project `create` service permission as the former ETL `_can_create`, reads `src_repo` from Gecko's registered project configuration, then requests a GitHub installation token from Fence with `access: read`. The registered repository must be on `github.com` and match the requested organization and project.

`AUTHENTICATE` validates the bearer token with Fence `/user/` and returns the verified `sub` to Funnel for task ownership. `GET` and `CANCEL` return an unchanged cloned config after Funnel checks ownership.

## Credential handling limits

The GitHub token and caller bearer token are passed through Funnel's per-task executor configuration and Kubernetes job environment. This avoids persisting either token in the TES task record. The deployment does not use Kubernetes Secrets. The tokens can therefore be visible to principals who can read the relevant worker configuration or job objects. Fence currently returns an installation token; the plugin validates the broker's repository identity and expiry response but cannot claim that Fence's token is scoped to one repository. The broker token expires after its reported lifetime, and this plugin does not refresh it if a task waits in the queue past expiry.

## Image build

Build this directory as the Funnel init image. The binary is available at `/funnel-gen3-auth`; the chart copies it to `/opt/funnel/plugin-binaries/auth-plugin`.

```sh
docker build -t <registry>/funnel-gen3-auth:<tag> .
```

`INPUT_DATA` contains only the project, immutable repository commit, and selected file paths. The GitHub token is supplied separately as `GITHUB_TOKEN`. Git-DRS acquisition runs in the ETL worker container; Loom receives the hydrated files through its HTTP ingestion client.
