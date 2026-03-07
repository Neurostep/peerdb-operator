# Performance Tuning

The operator exposes several flags for tuning performance at scale.

## Namespace Scoping

By default the operator watches all namespaces. To restrict it to a single namespace (reduces API server load and memory):

```sh
--watch-namespace=peerdb-production
```

## Leader Election Tuning

When running multiple replicas for HA, tune the leader election timing to balance failover speed vs API server churn:

| Flag | Default | Description |
|------|---------|-------------|
| `--leader-elect` | `false` | Enable leader election |
| `--leader-elect-lease-duration` | `15s` | Time a non-leader waits before forcing a new election |
| `--leader-elect-renew-deadline` | `10s` | Time the leader retries before giving up |
| `--leader-elect-retry-period` | `2s` | Interval between election retries |

**Trade-off:** Shorter lease durations mean faster failover but more API server traffic. For most deployments, the defaults are appropriate. Increase `lease-duration` to 30–60s in large clusters with many controllers competing for API server bandwidth.

## Informer Resync Period

```sh
--sync-period=10h
```

Controls how often informers re-list all objects from the API server. The default (controller-runtime's 10 hours) is appropriate for most cases. Only reduce this if you suspect the operator is missing watch events due to etcd compaction or network issues.

## Workqueue Rate Limiting

Each controller has built-in rate limiting (not configurable via flags):

| Controller | Max Concurrent | Backoff | Global Limit |
|------------|---------------|---------|--------------|
| PeerDBCluster | 1 | 1s–60s exponential | 10 qps, burst 100 |
| PeerDBWorkerPool | 2 | 1s–60s exponential | 10 qps, burst 100 |
| PeerDBSnapshotPool | 2 | 1s–60s exponential | 10 qps, burst 100 |

The PeerDBCluster controller is limited to 1 concurrent reconcile since it manages shared resources. Worker and snapshot pool controllers allow 2 concurrent reconciles since different pools are independent.

## Status Update Optimization

All controllers compare status before and after reconciliation and only write to the API server when status actually changed. This eliminates redundant status updates on steady-state reconciles, significantly reducing API server load when managing many clusters/pools.
