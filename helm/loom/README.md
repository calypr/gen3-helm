# Loom Helm chart

The chart deploys the Loom FHIR dataframe GraphQL service, one ClickHouse
StatefulSet, and the ArangoDB service it requires. No ClickHouse operator or
Keeper cluster is involved. The in-cluster endpoints are fixed at
`clickhouse:9000` and `arangodb:8529`.

## Local kind smoke test

```bash
cd /path/to/gen3-helm
export LOOM_REPO=/path/to/loom
docker build -t loom:dev "$LOOM_REPO"
kind create cluster --name loom
kind load docker-image loom:dev --name loom
helm upgrade --install loom ./helm/loom \
  --namespace loom --create-namespace \
  -f ./helm/loom/values-local.yaml
kubectl -n loom rollout status deployment/loom-deployment --timeout=5m
kubectl -n loom port-forward svc/loom 8080:8080
curl http://127.0.0.1:8080/health
```

The local values use `--no-auth`, an ephemeral ArangoDB, and an ephemeral
single-node ClickHouse. Set `clickstack.persistence.enabled: true` when the
ClickHouse data must survive pod replacement.

To increase ClickHouse capacity for wide dataframe loads, override the local
resource values before upgrading:

```bash
helm upgrade --install loom ./helm/loom \
  --namespace loom --create-namespace \
  -f ./helm/loom/values-local.yaml \
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
`/etc/loom/dataframer.json` and sets
`server.dataframer.recipe` to that path. Override the recipe without rebuilding
Loom:

This file is Loom's repository Explorer baseline. It is intentionally an
executable recipe only: its fields, dynamic columns, traversals, catalog
projections, and pivots define the live dataset shape. Do not add `views`,
filters, charts, table columns, or file actions here; those are presentation
owned by custom Builder Explorers. Loom derives default schema/readiness and
publication metadata from the materialized generation and exposes it through
the Explorer REST resource.

```bash
helm upgrade --install loom ./helm/loom \
  --namespace loom --create-namespace \
  -f ./helm/loom/values-local.yaml \
  --set-file dataframer.recipe=/path/to/dataframer.json
```

The recipe is omitted when `server.clickhouse.enabled` is false. Loom requires
the configured recipe when ClickHouse is enabled and fails startup if it is
missing or invalid.

Load a resource file through
the existing import API after port-forwarding, for example:

```bash
curl -F project=ARANGODB_PROTO \
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
