# PeerDB Operator — Production Installation Guide

This guide covers hardened deployment of PeerDB on Kubernetes with high availability, TLS, monitoring, network policies, and autoscaling.

## Prerequisites

| Requirement | Minimum Version | Notes |
|---|---|---|
| Kubernetes cluster | 1.26+ | Production-grade (EKS, GKE, AKS) |
| `kubectl` | matching cluster | |
| `helm` | 3.x | |
| PostgreSQL | 14+ | External, with TLS enabled |
| Temporal Server | 1.22+ | External, with mTLS recommended |
| cert-manager | 1.12+ | Required for webhook TLS certificate management |
| Prometheus Operator | 0.65+ | Optional, for ServiceMonitor support |

## 1. Namespace Setup

```bash
kubectl create namespace peerdb-system

# Label for network policy targeting
kubectl label namespace peerdb-system app.kubernetes.io/part-of=peerdb
```

## 2. RBAC Considerations

The operator uses a **ClusterRole** (`manager-role`) with the following permissions:

| API Group | Resources | Verbs |
|---|---|---|
| `""` (core) | configmaps, serviceaccounts, services | full CRUD + watch |
| `""` (core) | secrets | get, list, watch (read-only) |
| `""` (core) | events | create, patch |
| `apps` | deployments, statefulsets | full CRUD + watch |
| `batch` | jobs | full CRUD + watch |
| `peerdb.peerdb.io` | peerdbclusters, peerdbworkerpools, peerdbsnapshotpools | full CRUD + watch |
| `peerdb.peerdb.io` | `*/status`, `*/finalizers` | get, patch, update |

The ClusterRole is bound to the operator's ServiceAccount via a ClusterRoleBinding. If your security policy requires namespace-scoped RBAC, you can create a Role + RoleBinding per namespace instead — but the operator must be able to watch its CRDs cluster-wide.

Pre-built aggregate ClusterRoles are provided for user-facing access:

- `peerdbcluster-admin-role` / `peerdbcluster-editor-role` / `peerdbcluster-viewer-role`
- `peerdbworkerpool-admin-role` / `peerdbworkerpool-editor-role` / `peerdbworkerpool-viewer-role`
- `peerdbsnapshotpool-admin-role` / `peerdbsnapshotpool-editor-role` / `peerdbsnapshotpool-viewer-role`

## 3. Install the Operator with Production Values

Create a `values-production.yaml`:

```yaml
# -- Run 2 operator replicas for HA with leader election
replicaCount: 2

image:
  repository: ghcr.io/neurostep/peerdb-operator
  pullPolicy: IfNotPresent
  tag: "0.1.0"  # pin to a specific version

# -- Operator resource limits
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi

# -- Leader election (required for multi-replica)
leaderElection:
  enabled: true

# -- Pod security context
podSecurityContext:
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault

securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - "ALL"

# -- Anti-affinity: spread operator pods across nodes
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app.kubernetes.io/name: peerdb-operator
        topologyKey: kubernetes.io/hostname

# -- Enable webhooks with cert-manager
webhook:
  enabled: true
  port: 9443
  certManager:
    enabled: true
    issuerRef:
      kind: ClusterIssuer
      name: letsencrypt-prod  # your cert-manager issuer

# -- Enable metrics + ServiceMonitor
metrics:
  enabled: true
  bindAddress: ":8443"
  secure: true
  service:
    port: 8443
  serviceMonitor:
    enabled: true
    additionalLabels:
      release: prometheus  # match your Prometheus Operator selector
    interval: 30s
    scrapeTimeout: 10s
```

Install:

```bash
helm install peerdb-operator \
  oci://ghcr.io/neurostep/peerdb-operator/peerdb-operator \
  --namespace peerdb-system \
  -f values-production.yaml
```

## 4. Create Secrets

### Catalog database credentials

```bash
kubectl create secret generic peerdb-catalog-credentials \
  --namespace peerdb-system \
  --from-literal=password='STRONG_CATALOG_PASSWORD'
```

### PeerDB Server and UI passwords

```bash
kubectl create secret generic peerdb-credentials \
  --namespace peerdb-system \
  --from-literal=password='STRONG_SERVER_PASSWORD'

kubectl create secret generic peerdb-ui-nextauth \
  --namespace peerdb-system \
  --from-literal=secret="$(openssl rand -base64 32)"
```

### Temporal TLS credentials

If your Temporal server requires mTLS, create a TLS Secret:

```bash
kubectl create secret tls peerdb-temporal-tls \
  --namespace peerdb-system \
  --cert=temporal-client.crt \
  --key=temporal-client.key
```

## 5. PostgreSQL Catalog Hardening

Use `sslMode: verify-full` and separate credentials with minimal privileges:

```sql
-- On your PostgreSQL instance
CREATE USER peerdb_catalog WITH PASSWORD 'STRONG_CATALOG_PASSWORD';
CREATE DATABASE peerdb OWNER peerdb_catalog;

-- Restrict to only the required privileges
REVOKE ALL ON DATABASE peerdb FROM PUBLIC;
GRANT CONNECT ON DATABASE peerdb TO peerdb_catalog;
```

Reference in the PeerDBCluster spec:

```yaml
spec:
  dependencies:
    catalog:
      host: "catalog-postgresql.peerdb-system.svc.cluster.local"
      port: 5432
      database: "peerdb"
      user: "peerdb_catalog"
      passwordSecretRef:
        name: peerdb-catalog-credentials
        key: password
      sslMode: "verify-full"
```

Ensure the PostgreSQL server's CA certificate is trusted by the pods (mount it via `extraEnv` or a projected volume if needed).

## 6. Temporal TLS Setup

Reference the TLS Secret in the `temporal` dependency using `tlsSecretRef`. The Secret should contain `tls.crt` and `tls.key` entries:

```yaml
spec:
  dependencies:
    temporal:
      address: "temporal-frontend.temporal.svc.cluster.local:7233"
      namespace: "peerdb"
      tlsSecretRef:
        name: peerdb-temporal-tls
        key: tls.crt
```

## 7. Production PeerDBCluster

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBCluster
metadata:
  name: peerdb
  namespace: peerdb-system
spec:
  version: "v0.36.7"
  upgradePolicy: Manual  # control when upgrades happen
  dependencies:
    catalog:
      host: "catalog-postgresql.peerdb-system.svc.cluster.local"
      port: 5432
      database: "peerdb"
      user: "peerdb_catalog"
      passwordSecretRef:
        name: peerdb-catalog-credentials
        key: password
      sslMode: "verify-full"
    temporal:
      address: "temporal-frontend.temporal.svc.cluster.local:7233"
      namespace: "peerdb"
      tlsSecretRef:
        name: peerdb-temporal-tls
        key: tls.crt
  components:
    flowAPI:
      replicas: 2
      resources:
        requests:
          cpu: 250m
          memory: 256Mi
        limits:
          cpu: "1"
          memory: 512Mi
      service:
        type: ClusterIP
    peerDBServer:
      replicas: 2
      resources:
        requests:
          cpu: 250m
          memory: 256Mi
        limits:
          cpu: "1"
          memory: 512Mi
      service:
        type: ClusterIP
      passwordSecretRef:
        name: peerdb-credentials
        key: password
    ui:
      replicas: 2
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 500m
          memory: 256Mi
      service:
        type: ClusterIP
      passwordSecretRef:
        name: peerdb-credentials
        key: password
      nextAuthSecretRef:
        name: peerdb-ui-nextauth
        key: secret
  init:
    temporalNamespaceRegistration:
      enabled: true
    temporalSearchAttributes:
      enabled: true
```

```bash
kubectl apply -f peerdbcluster-production.yaml
```

## 8. Network Policies

### Allow UI ingress from specific CIDR

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: peerdb-ui-ingress
  namespace: peerdb-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: ui
      app.kubernetes.io/part-of: peerdb
  policyTypes:
    - Ingress
  ingress:
    - from:
        - ipBlock:
            cidr: 10.0.0.0/8  # replace with your corporate CIDR
      ports:
        - protocol: TCP
          port: 3000
```

### Restrict Flow API to namespace-internal traffic

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: peerdb-flow-api-ingress
  namespace: peerdb-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: flow-api
      app.kubernetes.io/part-of: peerdb
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector: {}  # same namespace only
      ports:
        - protocol: TCP
          port: 8112
        - protocol: TCP
          port: 8113
```

### Worker egress to Temporal and databases only

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: peerdb-worker-egress
  namespace: peerdb-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: flow-worker
      app.kubernetes.io/part-of: peerdb
  policyTypes:
    - Egress
  egress:
    # Temporal frontend
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: temporal
      ports:
        - protocol: TCP
          port: 7233
    # Catalog PostgreSQL
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: postgresql
      ports:
        - protocol: TCP
          port: 5432
    # Source/target databases (adjust CIDRs to match your data plane)
    - to:
        - ipBlock:
            cidr: 10.0.0.0/8
      ports:
        - protocol: TCP
          port: 5432
    # DNS
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
```

## 9. Multiple Worker Pools

Create separate pools for different workload profiles — for example, a high-memory pool for large tables and a standard pool for everything else:

### Standard worker pool

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: peerdb-workers-standard
  namespace: peerdb-system
spec:
  clusterRef: "peerdb"
  replicas: 3
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
    limits:
      cpu: "4"
      memory: "8Gi"
  nodeSelector:
    node-pool: compute
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          podAffinityTerm:
            labelSelector:
              matchLabels:
                app.kubernetes.io/component: flow-worker
                peerdb.io/pool: peerdb-workers-standard
            topologyKey: kubernetes.io/hostname
```

### High-memory worker pool

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: peerdb-workers-highmem
  namespace: peerdb-system
spec:
  clusterRef: "peerdb"
  replicas: 1
  temporalTaskQueue: "highmem-queue"
  resources:
    requests:
      cpu: "4"
      memory: "32Gi"
    limits:
      cpu: "8"
      memory: "32Gi"
  nodeSelector:
    node-pool: highmem
  tolerations:
    - key: "workload-type"
      operator: "Equal"
      value: "highmem"
      effect: "NoSchedule"
```

### Production snapshot pool

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBSnapshotPool
metadata:
  name: peerdb-snapshot
  namespace: peerdb-system
spec:
  clusterRef: "peerdb"
  replicas: 2
  storage:
    size: "50Gi"
    storageClassName: "gp3"  # use fast storage for snapshot I/O
  terminationGracePeriodSeconds: 1800  # 30 minutes for large snapshots
  resources:
    requests:
      cpu: "1"
      memory: "4Gi"
    limits:
      cpu: "2"
      memory: "4Gi"
  nodeSelector:
    node-pool: compute
```

> **Important:** StatefulSet `volumeClaimTemplates` are immutable. Changing `storage.size` or `storageClassName` after creation requires deleting the PeerDBSnapshotPool and recreating it with a new name.

## 10. HPA Configuration for Workers

The operator natively supports autoscaling. When `autoscaling.enabled` is `true`, the controller skips setting `spec.replicas` on the Deployment to avoid fighting with the HPA.

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: peerdb-workers-autoscaled
  namespace: peerdb-system
spec:
  clusterRef: "peerdb"
  replicas: 2  # used as initial replica count only
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
    limits:
      cpu: "4"
      memory: "8Gi"
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilization: 70
```

The operator creates an HPA targeting the worker Deployment. For KEDA-based autoscaling (e.g., scaling on Temporal task queue depth), disable the built-in autoscaling and create a KEDA `ScaledObject` targeting the Deployment directly.

## 11. Monitoring Setup

### ServiceMonitor

The Helm chart creates a ServiceMonitor when `metrics.serviceMonitor.enabled: true`. Ensure the `additionalLabels` match your Prometheus Operator's `serviceMonitorSelector`.

### Key Metrics to Watch

| Metric | Description | Alert Threshold |
|---|---|---|
| `controller_runtime_reconcile_total` | Reconciliation count by controller and result | Error rate > 5% |
| `controller_runtime_reconcile_errors_total` | Failed reconciliation count | Any sustained increase |
| `controller_runtime_reconcile_time_seconds` | Reconciliation latency | p99 > 30s |
| `workqueue_depth` | Controller work queue depth | Sustained > 10 |
| `workqueue_queue_duration_seconds` | Time items spend in the queue | p99 > 60s |

### Example PrometheusRule

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: peerdb-operator-alerts
  namespace: peerdb-system
  labels:
    release: prometheus
spec:
  groups:
    - name: peerdb-operator
      rules:
        - alert: PeerDBReconcileErrors
          expr: |
            rate(controller_runtime_reconcile_errors_total{controller=~"peerdb.*"}[5m]) > 0
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "PeerDB operator reconciliation errors"
            description: "Controller {{ $labels.controller }} has reconciliation errors."

        - alert: PeerDBClusterNotReady
          expr: |
            kube_customresource_peerdbcluster_status_conditions{type="Ready",status="True"} == 0
          for: 15m
          labels:
            severity: critical
          annotations:
            summary: "PeerDBCluster not ready"
            description: "PeerDBCluster {{ $labels.name }} has been not ready for 15 minutes."

        - alert: PeerDBWorkerPoolScaledToZero
          expr: |
            kube_customresource_peerdbworkerpool_status_replicas == 0
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "PeerDB worker pool has zero replicas"
            description: "PeerDBWorkerPool {{ $labels.name }} has zero replicas."
```

## 12. Backup Considerations

The PeerDB Operator itself is stateless — all state lives in the catalog PostgreSQL database and Temporal. Your backup strategy should focus on:

### Backup fencing

The operator supports a **backup fencing annotation** that prevents destructive operations (rolling restarts, upgrades, scale changes) during backups. This ensures the catalog database backup is consistent with the running state.

```bash
# 1. Check that the cluster is safe for backup
kubectl get peerdbcluster peerdb -n peerdb-system \
  -o jsonpath='{.status.conditions[?(@.type=="BackupSafe")].status}'
# Should return "True"

# 2. Activate backup fencing
kubectl annotate peerdbcluster peerdb -n peerdb-system \
  peerdb.io/backup-in-progress=true

# 3. Take your backup (e.g., pg_dump)
pg_dump -h <catalog-host> -U peerdb -d peerdb > catalog-backup.sql

# 4. Remove the fencing annotation
kubectl annotate peerdbcluster peerdb -n peerdb-system \
  peerdb.io/backup-in-progress-
```

While the annotation is set:
- The operator continues health monitoring and status updates
- Deployments, StatefulSets, and Jobs are **not mutated** (no rolling restarts, no upgrades, no scale changes)
- Worker and snapshot pool controllers also respect the annotation on the referenced cluster
- The `BackupSafe` condition is set to `False` with reason `BackupInProgress`

### Catalog database

- **What to back up:** The catalog PostgreSQL database contains peer definitions, mirror configurations, and flow state.
- **Strategy:** Use your PostgreSQL backup tooling (pg_dump, WAL archiving, or managed backup — e.g., AWS RDS automated backups). Activate backup fencing (see above) before taking a backup.
- **RPO target:** Losing catalog data means recreating peers and mirrors. Target RPO < 1 hour.

### Temporal

- **What to back up:** Temporal persistence stores workflow execution state.
- **Strategy:** Back up Temporal's underlying database (typically PostgreSQL or Cassandra). If using Temporal Cloud, backups are managed for you.

### Kubernetes resources

- **What to back up:** PeerDBCluster, PeerDBWorkerPool, and PeerDBSnapshotPool CRs, plus associated Secrets.
- **Strategy:** Use Velero or store manifests in Git (GitOps).

```bash
# Export current state
kubectl get peerdbclusters,peerdbworkerpools,peerdbsnapshotpools \
  -n peerdb-system -o yaml > peerdb-backup.yaml
```

### Snapshot worker PVCs

- Snapshot worker PVCs contain temporary data during initial loads. They do not need regular backup — they can be recreated by re-running the snapshot.

## Next Steps

- Review the [minimal installation guide](minimal.md) if you need a quick development setup.
- Check the [sample manifests](../../config/samples/) for additional configuration examples.
