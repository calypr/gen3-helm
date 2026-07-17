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
kubectl -n loom rollout status deployment/loom-loom --timeout=5m
kubectl -n loom port-forward svc/loom-loom 8080:8080
curl http://127.0.0.1:8080/healthz
```

The local values use `--no-auth`, an ephemeral ArangoDB, and an ephemeral
single-node ClickHouse. Set `clickstack.persistence.enabled: true` when the
ClickHouse data must survive pod replacement.
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
unless the endpoint is reachable from a BusyBox init container. The chart's
liveness/readiness probes use `/healthz`; the endpoint is process-level health
and does not hide backend connection failures during startup.

To run Loom without ClickHouse, set `server.clickhouse.enabled: false`. The
chart then emits an empty ClickHouse URL and omits the ClickHouse wait
init-container. The Loom image must include the matching disabled-backend
support.
