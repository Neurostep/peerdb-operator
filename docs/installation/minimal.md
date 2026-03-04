# PeerDB Operator — Minimal Installation Guide

This guide walks you through the quickest path to a running PeerDB cluster on Kubernetes using the PeerDB Operator.

## Prerequisites

| Requirement | Minimum Version | Notes |
|---|---|---|
| Kubernetes cluster | 1.26+ | Any conformant distribution (EKS, GKE, AKS, kind, etc.) |
| `kubectl` | matching cluster | Configured with cluster access |
| `helm` | 3.x | Only if installing via Helm |
| PostgreSQL | 14+ | External instance for the PeerDB catalog database |
| Temporal Server | 1.22+ | External instance; PeerDB uses it for workflow orchestration |

> **Note:** The operator does _not_ manage PostgreSQL or Temporal — you must provide these externally.

## 1. Install the Operator

### Option A: Helm (recommended)

```bash
# Add the OCI registry (no `helm repo add` needed for OCI)
helm install peerdb-operator \
  oci://ghcr.io/neurostep/peerdb-operator/peerdb-operator \
  --namespace peerdb-system \
  --create-namespace
```

To customise the installation, create a `values.yaml` override:

```yaml
# values-override.yaml
replicaCount: 1
resources:
  requests:
    cpu: 10m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 128Mi
```

```bash
helm install peerdb-operator \
  oci://ghcr.io/neurostep/peerdb-operator/peerdb-operator \
  --namespace peerdb-system \
  --create-namespace \
  -f values-override.yaml
```

### Option B: Plain manifests

```bash
kubectl apply -f https://raw.githubusercontent.com/Neurostep/peerdb-operator/main/install.yaml
```

This installs the CRDs, RBAC, and the operator Deployment into the `peerdb-system` namespace.

## 2. Create the Catalog Secret

Create a Kubernetes Secret containing the PostgreSQL password for the catalog database:

```bash
kubectl create namespace peerdb-system  # skip if namespace already exists

kubectl create secret generic peerdb-catalog-credentials \
  --namespace peerdb-system \
  --from-literal=password='YOUR_CATALOG_PASSWORD'
```

## 3. Create a PeerDBCluster

Apply the following manifest to deploy the PeerDB control plane (Flow API, PeerDB Server, and UI):

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBCluster
metadata:
  name: peerdb
  namespace: peerdb-system
spec:
  version: "v0.36.7"
  dependencies:
    catalog:
      host: "catalog-postgresql.peerdb-system.svc.cluster.local"
      port: 5432
      database: "peerdb"
      user: "peerdb"
      passwordSecretRef:
        name: peerdb-catalog-credentials
        key: password
      sslMode: "disable"
    temporal:
      address: "temporal-frontend.default.svc.cluster.local:7233"
      namespace: "default"
```

```bash
kubectl apply -f peerdbcluster.yaml
```

The operator will create:

- **Flow API** Deployment + Service (gRPC :8112, HTTP :8113)
- **PeerDB Server** Deployment + Service (Postgres wire protocol :9900)
- **PeerDB UI** Deployment + Service (HTTP :3000)
- **Init Jobs** for Temporal namespace registration and search attribute setup

## 4. Create a PeerDBWorkerPool

Workers run CDC flows. They are CPU/memory intensive and scale independently from the control plane:

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: peerdb-workers
  namespace: peerdb-system
spec:
  clusterRef: "peerdb"
  replicas: 2
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
    limits:
      cpu: "4"
      memory: "8Gi"
```

```bash
kubectl apply -f peerdbworkerpool.yaml
```

## 5. Create a PeerDBSnapshotPool

Snapshot workers handle initial data loads. They are bursty — scale up during loads, scale to zero when idle:

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBSnapshotPool
metadata:
  name: peerdb-snapshot
  namespace: peerdb-system
spec:
  clusterRef: "peerdb"
  replicas: 1
  storage:
    size: "10Gi"
  terminationGracePeriodSeconds: 600
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "1"
      memory: "1Gi"
```

```bash
kubectl apply -f peerdbsnapshotpool.yaml
```

> **Note:** Snapshot workers run as a StatefulSet. The `storage.size` field provisions a PersistentVolumeClaim per replica. `volumeClaimTemplates` are immutable in Kubernetes — changing `storage.size` after creation requires deleting and recreating the pool.

## 6. Verify the Installation

### Check the PeerDBCluster status

```bash
kubectl get peerdbclusters -n peerdb-system
```

Expected output:

```
NAME     READY   VERSION   AGE
peerdb   True    v0.36.7   2m
```

### Inspect conditions

```bash
kubectl describe peerdbcluster peerdb -n peerdb-system
```

Look for these conditions — all should show `Status: True` for a healthy cluster:

| Condition | Meaning |
|---|---|
| `CatalogReady` | PostgreSQL catalog connection is healthy |
| `TemporalReady` | Temporal connection is healthy |
| `Initialized` | Init jobs (namespace registration, search attributes) completed |
| `ComponentsReady` | All Deployments (Flow API, Server, UI) are ready |
| `Ready` | Overall cluster readiness |

### Check worker pools

```bash
kubectl get peerdbworkerpools -n peerdb-system
kubectl get peerdbsnapshotpools -n peerdb-system
```

### Check pods

```bash
kubectl get pods -n peerdb-system
```

You should see pods for: the operator, Flow API, PeerDB Server, UI, flow workers, and snapshot workers.

## 7. Access the PeerDB UI

By default the UI Service uses `ClusterIP`. Use port-forwarding to access it locally:

```bash
kubectl port-forward svc/peerdb-ui 3000:3000 -n peerdb-system
```

Then open [http://localhost:3000](http://localhost:3000) in your browser.

To expose the UI externally, set the service type to `LoadBalancer`:

```yaml
spec:
  components:
    ui:
      service:
        type: LoadBalancer
```

Then retrieve the external address:

```bash
kubectl get svc peerdb-ui -n peerdb-system -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'
```

## Next Steps

- See the [Production Installation Guide](production.md) for HA, TLS, monitoring, and network policies.
- Check the [sample manifests](../../config/samples/) for more configuration examples.
