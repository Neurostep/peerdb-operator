# Runbook: Backup Awareness

The PeerDB Operator supports **backup fencing** — a mechanism that prevents destructive reconciliation (rolling restarts, upgrades, scale changes) while a catalog database backup is in progress. This ensures backup consistency without fully stopping the operator.

## How It Works

The operator checks for the annotation `peerdb.io/backup-in-progress` on the `PeerDBCluster` resource. When present:

| What continues | What is fenced (skipped) |
|---|---|
| Health monitoring and status updates | Deployment/StatefulSet mutations |
| Dependency validation (catalog secret, Temporal config) | Version upgrades |
| ServiceAccount and ConfigMap reconciliation | Init job creation |
| `BackupSafe` condition reporting | Replica scaling |
| Worker/snapshot pool status reporting | Rolling restarts from config changes |

The `BackupSafe` condition on the cluster reports whether it's safe to take a backup:

| Status | Reason | Meaning |
|--------|--------|---------|
| `True` | `BackupSafe` | No upgrade or rollout in progress — safe to backup |
| `False` | `BackupInProgress` | Fencing annotation is set — backup presumed in progress |
| `False` | `BackupUnsafe` | Upgrade or rolling restart in progress — wait before backing up |

## Backup Procedure

### 1. Check backup safety

```bash
kubectl get peerdbcluster <name> -n <namespace> \
  -o jsonpath='{.status.conditions[?(@.type=="BackupSafe")]}'
```

Verify `status: "True"`. If `False` with reason `BackupUnsafe`, wait for the current upgrade or rollout to complete.

### 2. Activate fencing

```bash
kubectl annotate peerdbcluster <name> -n <namespace> \
  peerdb.io/backup-in-progress=true
```

The operator will emit a `BackupInProgress` event and skip all destructive operations on the next reconciliation loop.

### 3. Take the backup

Back up the catalog PostgreSQL database using your preferred tool:

```bash
# pg_dump
pg_dump -h <catalog-host> -U peerdb -d peerdb > catalog-backup.sql

# Or use managed backup (RDS snapshot, CloudSQL export, etc.)
```

If using Velero for full cluster backup:

```bash
velero backup create peerdb-backup \
  --include-namespaces peerdb-system \
  --wait
```

### 4. Remove fencing

```bash
kubectl annotate peerdbcluster <name> -n <namespace> \
  peerdb.io/backup-in-progress-
```

The operator will resume normal reconciliation immediately.

## Scope

The fencing annotation on a `PeerDBCluster` is respected by:

- **PeerDBCluster controller** — skips upgrades, init jobs, and component Deployment updates
- **PeerDBWorkerPool controller** — skips Deployment mutations for all worker pools referencing the fenced cluster
- **PeerDBSnapshotPool controller** — skips StatefulSet mutations for all snapshot pools referencing the fenced cluster

## Automated Backup Scripts

You can integrate fencing into a cron job or CI pipeline:

```bash
#!/bin/bash
set -euo pipefail

CLUSTER=peerdb
NAMESPACE=peerdb-system
CATALOG_HOST=catalog-postgresql.peerdb-system.svc.cluster.local

# Wait until safe
while true; do
  safe=$(kubectl get peerdbcluster "$CLUSTER" -n "$NAMESPACE" \
    -o jsonpath='{.status.conditions[?(@.type=="BackupSafe")].status}')
  [ "$safe" = "True" ] && break
  echo "Waiting for cluster to be backup-safe..."
  sleep 10
done

# Fence
kubectl annotate peerdbcluster "$CLUSTER" -n "$NAMESPACE" \
  peerdb.io/backup-in-progress=true --overwrite

# Backup
pg_dump -h "$CATALOG_HOST" -U peerdb -d peerdb > "catalog-$(date +%Y%m%d-%H%M%S).sql"

# Unfence
kubectl annotate peerdbcluster "$CLUSTER" -n "$NAMESPACE" \
  peerdb.io/backup-in-progress-
```

## Comparison with `spec.paused`

| Feature | `spec.paused: true` | `peerdb.io/backup-in-progress` |
|---|---|---|
| Health monitoring | ❌ Stopped | ✅ Continues |
| Status updates | Minimal (sets Ready=False) | ✅ Full status reporting |
| Dependency validation | ❌ Skipped | ✅ Continues |
| Upgrades | ❌ Skipped | ❌ Skipped |
| Component mutations | ❌ Skipped | ❌ Skipped |
| Requires spec change | Yes (triggers new generation) | No (annotation only) |
| Intended use | Full operator pause | Temporary backup window |
