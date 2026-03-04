# PeerDB Operator Troubleshooting Guide

## Conditions Reference

### PeerDBCluster Conditions

| Condition | Reason | Meaning | Action |
|-----------|--------|---------|--------|
| `Ready=False` | `DependencyNotReady` | Catalog secret or Temporal config missing | Create the referenced Secret or fix dependency configuration |
| `Ready=False` | `Paused` | Reconciliation is paused | Set `spec.paused: false` to resume |
| `Ready=False` | `ClusterNotReady` | One or more subsystems not healthy | Inspect other conditions to find the root cause |
| `CatalogReady=False` | `SecretNotFound` | Catalog password Secret not found | Create Secret with the name and key specified in `spec.dependencies.catalog.passwordSecretRef` |
| `CatalogReady=False` | `DependencyNotReady` | Catalog DB unreachable or misconfigured | Verify `spec.dependencies.catalog` host, port, database, and user |
| `CatalogReady=True` | `SecretFound` | Catalog secret is available | No action needed |
| `CatalogReady=True` | `Configured` | Catalog is fully configured | No action needed |
| `TemporalReady=False` | `DependencyNotReady` | Temporal config missing or unreachable | Verify `spec.dependencies.temporal.address` is a valid `host:port` |
| `TemporalReady=False` | `SecretNotFound` | Temporal TLS Secret not found | Create the TLS Secret referenced by `spec.dependencies.temporal.tlsSecretRef` |
| `TemporalReady=True` | `Configured` | Temporal is configured | No action needed |
| `Initialized=False` | `JobsPending` | Init jobs still running | Wait for completion, or check job logs if stuck |
| `Initialized=False` | `JobFailed` | An init job has failed | Check job logs, fix config, delete the failed job to retry. See [Init Jobs Failing](runbooks/init-jobs-failing.md) |
| `Initialized=True` | `JobsCompleted` | All init jobs completed | No action needed |
| `ComponentsReady=False` | `ComponentsNotReady` | Some Deployments are not ready | Check deployment events and pod status |
| `ComponentsReady=True` | `AllReady` | All components are running | No action needed |
| `Degraded=True` | `Degraded` | Cluster running in a degraded state | Check individual component conditions and pod status |
| `Reconciling=True` | `Reconciling` | Controller is actively reconciling | Wait for reconciliation to complete |
| `Reconciling=False` | `ReconcileComplete` | Reconciliation completed | No action needed |
| `UpgradeInProgress=True` | `UpgradeInProgress` | Version upgrade is in progress | Monitor upgrade phase via `status.upgrade` |
| `UpgradeInProgress=True` | `UpgradeBlocked` | Upgrade blocked by unhealthy dependencies | Fix dependencies before upgrade can proceed |
| `UpgradeInProgress=True` | `MaintenanceWindow` | Waiting for maintenance window | Upgrade will proceed during the configured window, or remove `spec.maintenanceWindow` |
| `UpgradeInProgress=True` | `VersionSkew` | Version skew detected between components | Wait for ordered rollout to complete, or check for stuck components |
| `UpgradeInProgress=False` | `UpgradeComplete` | Upgrade completed successfully | No action needed |
| `BackupSafe=False` | `BackupInProgress` | Backup fencing is active (annotation set) | Destructive operations are fenced. Remove `peerdb.io/backup-in-progress` annotation when backup completes |
| `BackupSafe=False` | `BackupUnsafe` | Upgrade or rolling restart in progress | Wait for rollout to complete before taking a backup |
| `BackupSafe=True` | `BackupSafe` | Cluster is safe for backup | Safe to take a catalog DB backup |

### PeerDBWorkerPool Conditions

| Condition | Reason | Meaning | Action |
|-----------|--------|---------|--------|
| `Ready=False` | `ClusterNotFound` | Referenced PeerDBCluster does not exist | Create the PeerDBCluster or fix `spec.clusterRef` |
| `Ready=False` | `ClusterNotReady` | Referenced PeerDBCluster is not ready | Wait for the cluster to become Ready, or troubleshoot the cluster |
| `Ready=False` | `DeploymentNotReady` | Worker Deployment not ready | Check pod events, resource constraints, image pull errors |
| `Ready=False` | `DeploymentCreated` | Deployment was just created | Wait for pods to start (requeued in ~5s) |
| `Ready=True` | `DeploymentReady` | Worker Deployment is fully ready | No action needed |
| `Available=False` | `DeploymentNotReady` | No worker replicas available | Check pod scheduling, resource limits, node capacity |
| `Available=True` | `DeploymentReady` | At least one worker replica is available | No action needed |

### PeerDBSnapshotPool Conditions

| Condition | Reason | Meaning | Action |
|-----------|--------|---------|--------|
| `Ready=False` | `ClusterNotFound` | Referenced PeerDBCluster does not exist | Create the PeerDBCluster or fix `spec.clusterRef` |
| `Ready=False` | `ClusterNotReady` | Referenced PeerDBCluster is not ready | Wait for the cluster to become Ready |
| `Ready=False` | `StatefulSetNotReady` | StatefulSet not ready | Check PVC binding, pod events, storage class availability |
| `Ready=False` | `StatefulSetCreated` | StatefulSet was just created | Wait for pods to start |
| `Ready=True` | `StatefulSetReady` | Snapshot StatefulSet is fully ready | No action needed |
| `Available=False` | `StatefulSetNotReady` | No snapshot replicas available | Check PVC provisioning, pod scheduling, node capacity |
| `Available=True` | `StatefulSetReady` | At least one snapshot worker is available | No action needed |

## Common Events

The operator records Kubernetes events on the CR for significant lifecycle transitions:

| Event Type | Reason | Description |
|------------|--------|-------------|
| `Warning` | `SecretNotFound` | A required Secret was not found. The event message includes the Secret name and key. |
| `Normal` | `SecretFound` | A previously missing Secret was found. |
| `Normal` | `JobsCompleted` | All init jobs completed successfully. |
| `Warning` | `JobFailed` | An init job failed. Check job logs for details. |
| `Normal` | `JobsPending` | Init jobs are running. |
| `Normal` | `DeploymentCreated` | A component Deployment was created. |
| `Normal` | `DeploymentReady` | A component Deployment reached ready state. |
| `Warning` | `DeploymentNotReady` | A component Deployment is not ready. |
| `Normal` | `StatefulSetCreated` | A snapshot StatefulSet was created. |
| `Normal` | `StatefulSetReady` | A snapshot StatefulSet reached ready state. |
| `Warning` | `StatefulSetNotReady` | A snapshot StatefulSet is not ready. |
| `Normal` | `ReconcileComplete` | Reconciliation completed successfully. |
| `Normal` | `Paused` | Reconciliation was paused. |
| `Normal` | `BackupInProgress` | Backup fencing activated — destructive operations skipped. |
| `Normal` | `UpgradeInProgress` | A version upgrade has started. |
| `Normal` | `UpgradeComplete` | A version upgrade completed successfully. |
| `Warning` | `UpgradeBlocked` | An upgrade is blocked by unhealthy dependencies. |
| `Warning` | `Degraded` | The cluster entered a degraded state. |

### Viewing Events

```bash
# Events for a specific resource
kubectl get events --field-selector involvedObject.name=<resource-name> --sort-by='.lastTimestamp'

# All PeerDB-related events
kubectl get events --sort-by='.lastTimestamp' | grep -i peerdb

# Watch events in real-time
kubectl get events -w --field-selector involvedObject.name=<resource-name>
```

## Useful kubectl Commands

### Cluster Status

```bash
# Quick overview of all PeerDB resources
kubectl get peerdbcluster,peerdbworkerpool,peerdbsnapshotpool

# Detailed cluster conditions
kubectl get peerdbcluster <name> -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'

# Cluster endpoints
kubectl get peerdbcluster <name> -o jsonpath='{.status.endpoints}' | jq .

# Upgrade status
kubectl get peerdbcluster <name> -o jsonpath='{.status.upgrade}' | jq .
```

### Component Health

```bash
# All managed Deployments
kubectl get deployments -l app.kubernetes.io/instance=<cluster-name>

# All managed pods with their status
kubectl get pods -l app.kubernetes.io/instance=<cluster-name> -o wide

# Init jobs
kubectl get jobs -l app.kubernetes.io/instance=<cluster-name>

# Services
kubectl get svc -l app.kubernetes.io/instance=<cluster-name>
```

### Worker Pool Diagnostics

```bash
# Worker pool status
kubectl get peerdbworkerpool <name> -o jsonpath='{.status}' | jq .

# Worker pods
kubectl get pods -l app.kubernetes.io/instance=<cluster-name>,app.kubernetes.io/component=flow-worker

# HPA (if autoscaling enabled)
kubectl get hpa -l app.kubernetes.io/instance=<cluster-name>
```

### Snapshot Pool Diagnostics

```bash
# Snapshot pool status
kubectl get peerdbsnapshotpool <name> -o jsonpath='{.status}' | jq .

# Snapshot pods
kubectl get pods -l app.kubernetes.io/instance=<cluster-name>,app.kubernetes.io/component=snapshot-worker

# PVCs for snapshot workers
kubectl get pvc -l app.kubernetes.io/instance=<cluster-name>,app.kubernetes.io/component=snapshot-worker
```

### ConfigMap and Secrets

```bash
# View the shared config (non-sensitive values)
kubectl get configmap <cluster-name>-config -o yaml

# List referenced secrets
kubectl get secrets -l app.kubernetes.io/instance=<cluster-name>
```

## Checking Operator Logs

### Basic Log Access

```bash
# View operator logs
kubectl logs -l app.kubernetes.io/name=peerdb-operator -n <operator-ns>

# Follow logs in real-time
kubectl logs -f -l app.kubernetes.io/name=peerdb-operator -n <operator-ns>

# Filter for a specific cluster
kubectl logs -l app.kubernetes.io/name=peerdb-operator -n <operator-ns> | grep <cluster-name>

# Filter for errors
kubectl logs -l app.kubernetes.io/name=peerdb-operator -n <operator-ns> | grep -i error
```

### Increase Log Verbosity

The operator uses controller-runtime's `zap` logger. To increase verbosity, set the `-zap-log-level` flag on the operator deployment:

```bash
# Edit the operator Deployment
kubectl edit deployment peerdb-operator-controller-manager -n <operator-ns>
```

Add or modify the `--zap-log-level` argument in the container args:

```yaml
containers:
  - name: manager
    args:
      - "--zap-log-level=debug"    # Options: debug, info (default), error
      - "--zap-devel=true"         # Enables development mode (human-readable output)
```

Or use a numeric level for fine-grained control:

```yaml
args:
  - "--zap-log-level=5"  # Higher numbers = more verbose
```

After editing, the operator pod will restart automatically.

### Log Context

Operator log entries include structured fields that help with filtering:

- `controller`: The controller name (`peerdbcluster`, `peerdbworkerpool`, `peerdbsnapshotpool`)
- `namespace`: The namespace of the resource being reconciled
- `name`: The name of the resource being reconciled

Example log entry:
```
{"level":"info","ts":"2026-03-04T12:00:00Z","controller":"peerdbcluster","namespace":"default","name":"peerdb","msg":"reconciliation complete"}
```

## Runbooks

For detailed resolution guides, see the runbooks:

- [Cluster Stuck NotReady](runbooks/cluster-not-ready.md)
- [Init Jobs Failing](runbooks/init-jobs-failing.md)
- [Scaling Worker Pools](runbooks/scaling-worker-pools.md)
- [Safe Upgrade](runbooks/safe-upgrade.md)
