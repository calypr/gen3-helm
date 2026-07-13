# redis

In-cluster Redis used as the shared Fence authz snapshot cache backend.

Primary consumer:

- `AUTHZ_SNAPSHOT_CACHE_REDIS_URL=redis://authz-cache-service:6379/0`

Deployment notes:

- enabled through the umbrella `gen3` chart with `redis.enabled`
- exposed only as an internal Kubernetes `Service`
- intended for local/dev Kubernetes use first

Current defaults:

- single replica
- password auth enabled through a Kubernetes secret
- no persistence
- ingress restricted to Fence pods when network policies are enabled

Future hardening can add:

- persistence
- StatefulSet semantics
- egress policy
