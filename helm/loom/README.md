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
materialized tables in its built-in `default` database.

Both databases use pod-local storage by default. To retain data across pod
replacement, enable persistence. The following Gen3 values request 30 GiB for
ArangoDB and 6 GiB for ClickHouse from the cluster's default StorageClass.

```yaml
loom:
  arango:
    persistence:
      enabled: true
  clickstack:
    persistence:
      enabled: true
```

Set `storageClass` under either persistence block to select a named class.
The chart creates the ArangoDB claim. The ClickHouse StatefulSet creates its
claim. Set `existingClaim` only when mounting a claim that already exists in
the release namespace. The chart retains its ArangoDB claim when the release
is uninstalled. ArangoDB replaces its pod before starting another pod on the
same claim.

Back up existing pod-local data before switching storage, then restore it
into the new volumes. If ClickHouse already uses a StatefulSet claim template,
keep that mode for in-place upgrades. Switching away from it requires a
StatefulSet migration.

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
Loom registers the recipe at startup. Publishing a project generation runs the
recipe and creates the initial project view from the published outputs.

To override the recipe, pass
`--set-file dataframer.recipe=/path/to/dataframer.json`.
The chart mounts the recipe only when ClickHouse is enabled.

For older Loom images that support `server.project_configs`, explicitly set
`projectConfigs.enabled=true` to mount the legacy viewer catalog. To supply
a custom catalog, pass
`--set-file projectConfigs.definitions=/path/to/project-configs.json`.

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
