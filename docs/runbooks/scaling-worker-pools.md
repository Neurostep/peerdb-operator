# Runbook: How to Scale Worker Pools

## Overview

PeerDB uses two types of worker pools with different scaling characteristics:

- **PeerDBWorkerPool** — CDC Flow Workers (Deployments). CPU/memory heavy, steady workload. Support HPA/KEDA.
- **PeerDBSnapshotPool** — Snapshot Workers (StatefulSets). Bursty workload — scale up for initial loads, scale to zero when idle.

## Manual Scaling

### Scale Worker Pool Replicas

```bash
kubectl patch peerdbworkerpool <name> --type merge -p '{"spec":{"replicas":4}}'
```

### Scale to Zero

```bash
kubectl patch peerdbworkerpool <name> --type merge -p '{"spec":{"replicas":0}}'
```

### Check Current Scale

```bash
kubectl get peerdbworkerpool
```

```
NAME              READY   REPLICAS   AGE
peerdb-workers    True    2          1d
```

## HPA-Based Autoscaling

### Enable Autoscaling

Configure autoscaling in the PeerDBWorkerPool spec:

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: peerdb-workers
spec:
  clusterRef: "peerdb"
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilization: 70
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
    limits:
      cpu: "4"
      memory: "16Gi"
```

### Important Notes

- **When `autoscaling.enabled=true`, the controller skips setting `spec.replicas`** on the Deployment to avoid fighting with the HPA. The HPA controls the replica count.
- **Resource requests are required** for CPU-based HPA to work. The HPA calculates utilization as a percentage of the requested CPU.
- The operator creates an HPA resource targeting the worker Deployment.
- `minReplicas` must be ≤ `maxReplicas` (enforced by webhook validation).

### Verify HPA

```bash
kubectl get hpa -l app.kubernetes.io/instance=<cluster-name>
kubectl describe hpa <hpa-name>
```

## KEDA Integration

For more advanced autoscaling (e.g., based on Temporal task queue depth), use KEDA with a ScaledObject targeting the worker Deployment:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: peerdb-workers-keda
spec:
  scaleTargetRef:
    name: <cluster-name>-flow-worker  # Deployment name created by the operator
  minReplicaCount: 1
  maxReplicaCount: 20
  triggers:
    - type: temporal-task-queue
      metadata:
        # Configure based on your Temporal setup
        address: "<temporal-address>"
        namespace: "<temporal-namespace>"
        taskQueue: "<task-queue-name>"
```

When using KEDA, set `autoscaling.enabled: true` in the PeerDBWorkerPool spec to prevent the operator from managing the replica count.

## Multiple Worker Pools

You can create multiple PeerDBWorkerPool resources referencing the same PeerDBCluster for different workload profiles:

```yaml
# IO-optimized pool for large table replication
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: peerdb-workers-io
spec:
  clusterRef: "peerdb"
  replicas: 3
  nodeSelector:
    node.kubernetes.io/instance-type: i3.xlarge
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
---
# Compute-optimized pool for transformation-heavy flows
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: peerdb-workers-compute
spec:
  clusterRef: "peerdb"
  replicas: 2
  nodeSelector:
    node.kubernetes.io/instance-type: c6g.2xlarge
  resources:
    requests:
      cpu: "4"
      memory: "4Gi"
  tolerations:
    - key: "workload"
      operator: "Equal"
      value: "compute"
      effect: "NoSchedule"
```

Use `temporalTaskQueue` to route different flows to different pools:

```yaml
spec:
  temporalTaskQueue: "io-heavy-flows"
```

## Snapshot Pool Scaling

### Scale Up for Initial Loads

```bash
kubectl patch peerdbsnapshotpool <name> --type merge -p '{"spec":{"replicas":5}}'
```

### Scale to Zero When Idle

```bash
kubectl patch peerdbsnapshotpool <name> --type merge -p '{"spec":{"replicas":0}}'
```

### Storage Considerations

> **⚠️ StatefulSet `volumeClaimTemplates` are immutable.** You cannot change the storage size or storage class of an existing snapshot pool. To change storage configuration, you must:
>
> 1. Create a new PeerDBSnapshotPool with the desired storage settings.
> 2. Scale the old pool to zero.
> 3. Delete the old pool (PVCs will be cleaned up via OwnerReferences).

Example snapshot pool with storage:

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBSnapshotPool
metadata:
  name: peerdb-snapshots
spec:
  clusterRef: "peerdb"
  replicas: 2
  storage:
    size: 100Gi
    storageClassName: gp3
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
  terminationGracePeriodSeconds: 600  # Allow long snapshots to complete
```

### Checking Snapshot Pool Status

```bash
kubectl get peerdbsnapshotpool
```

```
NAME               READY   REPLICAS   AGE
peerdb-snapshots   True    2          1d
```

Check PVC binding:

```bash
kubectl get pvc -l app.kubernetes.io/instance=<cluster-name>,app.kubernetes.io/component=snapshot-worker
```
