# Loom Helm chart

The chart deploys the Loom FHIR dataframe GraphQL service, one ClickHouse
StatefulSet, and the ArangoDB service it requires. No ClickHouse operator or
Keeper cluster is involved. The in-cluster endpoints are fixed at
`clickhouse:9000` and `arangodb:8529`.

## Local kind smoke test

Set `GEN3_HOSTNAME` to the hostname of your Fence deployment. The hostname
must match the issuer in the Gen3 access tokens used for testing.
For Loom builds that require population mapping cursor signing, set
`config.server.population_mapping_cursor_secret` in a private values file.
Use a stable random secret of at least 16 characters.

```bash
cd /path/to/gen3-helm
export LOOM_REPO=/path/to/loom
docker build -t loom:dev "$LOOM_REPO"
kind create cluster --name loom
kind load docker-image loom:dev --name loom
helm upgrade --install loom ./helm/loom \
  --namespace loom --create-namespace \
  -f ./helm/loom/values-local.yaml \
  --set-string global.hostname="$GEN3_HOSTNAME"
kubectl -n loom rollout status deployment/loom-deployment --timeout=5m
kubectl -n loom port-forward svc/loom 8080:8080
curl http://127.0.0.1:8080/health
```

The local values inherit Gen3/Fence auth and deploy both ArangoDB and
ClickHouse. ArangoDB stores the graph in `fhir_proto`. ClickHouse stores
materialized tables in its built-in `default` database. Set
`clickstack.persistence.enabled: true` when the ClickHouse data must survive
pod replacement.

To increase ClickHouse capacity for wide dataframe loads, override the local
resource values before upgrading:

```bash
helm upgrade --install loom ./helm/loom \
  --namespace loom --create-namespace \
  -f ./helm/loom/values-local.yaml \
  --set-string global.hostname="$GEN3_HOSTNAME" \
  --set clickstack.resources.requests.memory=4Gi \
  --set clickstack.resources.limits.memory=8Gi
```

The chart applies `clickstack.resources` directly to the ClickHouse
StatefulSet. The pod limit must be increased before ClickHouse can use the
additional memory.

The bundled ArangoDB defaults to `arangodb:3.12`. Override its image without
changing the Loom image:

```yaml
loom:
  arango:
    image:
      repository: quay.io/ohsu-comp-bio/arangodb-community
      tag: your-tag
      pullPolicy: Always
```

When ClickHouse is enabled, the chart mounts
[`files/default-dataframer.json`](files/default-dataframer.json) at
`/etc/loom/dataframer.json` and sets `server.dataframer.recipe` to that path.
The recipe contains dataframe definitions only. Viewer presentation lives in
the independent project-config catalog at
[`files/default-project-configs.json`](files/default-project-configs.json),
mounted at `/etc/loom/project-configs.json` and selected by
`server.project_configs` when `projectConfigs.enabled` is true. Each config
references the recipe by name and owns its views, table columns, filters, and
charts. Configs for one project generation share the recipe's single published
execution; they do not copy the recipe or run it separately. A new source
publication activates updated project-config definitions against its exact
generation and execution.

Override the recipe and project-config catalog without rebuilding Loom:

```bash
helm upgrade --install loom ./helm/loom \
  --namespace loom --create-namespace \
  -f ./helm/loom/values-local.yaml \
  --set-string global.hostname="$GEN3_HOSTNAME" \
  --set-file dataframer.recipe=/path/to/dataframer.json \
  --set-file projectConfigs.definitions=/path/to/project-configs.json
```

The bundled project-config catalog references `calypr-meta-default`. If the
recipe name changes, provide a matching catalog with
`projectConfigs.definitions`; disable project configs with
`--set projectConfigs.enabled=false` for a headless/custom-recipe deployment.
The recipe file and `server.dataframer.recipe` are omitted when ClickHouse is
disabled. The project-config file and `server.project_configs` are omitted when
ClickHouse or project configs are disabled. Loom requires the configured recipe
and any enabled config references to be valid at startup.

Load a resource file through
the existing import API after port-forwarding, for example:

```bash
curl -H "Authorization: Bearer $GEN3_ACCESS_TOKEN" \
  -F project=ARANGODB_PROTO \
  -F resource_type=Patient \
  -F use_generic=true \
  -F file=@"$LOOM_REPO/META/Patient.ndjson" \
  http://127.0.0.1:8080/api/v1/imports
```

For a real cluster, use a managed Loom image registry. If the Loom server is
configured to use ClickHouse, set `server.clickhouse.url` (or
`server.clickhouse.host` and `port`) and set `server.waitForBackends` false
unless the endpoint is reachable from a BusyBox init container. The chart uses
`/livez` for process liveness and `/readyz` for dependency-aware readiness;
override `probes.liveness.path` or `probes.readiness.path` only for an older
Loom image that does not expose the split health endpoints.

To run Loom without ClickHouse, set `server.clickhouse.enabled: false`. The
chart then emits an empty ClickHouse URL and omits the ClickHouse wait
init-container. The Loom image must include the matching disabled-backend
support.
