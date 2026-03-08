# Runbook: How to Perform a Safe Upgrade

## Pre-Upgrade Checklist

Before upgrading, verify:

1. **All conditions are healthy:**
   ```bash
   kubectl get peerdbcluster <name> -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\n"}{end}'
   ```
   Confirm `Ready=True` and no `Degraded=True`.

2. **No active initial loads** (or pause them first):
   ```bash
   kubectl get peerdbsnapshotpool -l app.kubernetes.io/instance=<cluster-name>
   ```
   Ideally, snapshot pools should be at zero replicas or idle before upgrading.

3. **Check release notes** for the target version for breaking changes or required migration steps.

4. **Worker pools are healthy:**
   ```bash
   kubectl get peerdbworkerpool
   kubectl get peerdbsnapshotpool
   ```
   Ensure all pools show `Ready=True`.

## Upgrade Methods

### Automatic Upgrade (Default)

Change the `spec.version` field and the controller handles the ordered rollout:

```bash
kubectl patch peerdbcluster <name> --type merge -p '{"spec":{"version":"v0.37.0"}}'
```

Or edit the resource:

```bash
kubectl edit peerdbcluster <name>
```

The controller will begin the upgrade immediately (or at the next maintenance window, if configured).

### Manual Upgrade

For more control, use the manual upgrade policy:

1. Set the upgrade policy to Manual:
   ```yaml
   spec:
     upgradePolicy: Manual
   ```

2. Change the version:
   ```bash
   kubectl patch peerdbcluster <name> --type merge -p '{"spec":{"version":"v0.37.0"}}'
   ```

3. The controller will detect the version change but **will not proceed** until the policy is set back to Automatic.

4. When ready, approve the upgrade:
   ```bash
   kubectl patch peerdbcluster <name> --type merge -p '{"spec":{"upgradePolicy":"Automatic"}}'
   ```

## Upgrade Order

The controller enforces a specific rollout order to minimize disruption:

```
[StartMaintenance →] ConfigMap/Secrets → Init Jobs → Flow API → PeerDB Server → UI [→ EndMaintenance]
```

Each step must complete successfully before the next begins. This ensures:
- Mirrors are gracefully paused before any component restarts (when `spec.maintenance` is configured).
- Configuration is propagated before any component restarts.
- The Flow API (gRPC backend) is ready before the Server and UI that depend on it.
- The UI is upgraded last since it's the least critical component.

## Maintenance Window

Configure a daily window during which upgrades may start:

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBCluster
metadata:
  name: peerdb
spec:
  version: "v0.37.0"
  upgradePolicy: Automatic
  maintenanceWindow:
    start: "02:00"
    end: "06:00"
    timeZone: "America/Los_Angeles"
  # ... rest of spec
```

- Upgrades will only **start** during the window. An upgrade that begins within the window will complete even if it runs past the end time.
- The maintenance window is only used when `upgradePolicy` is `Automatic`.
- Remove or omit `maintenanceWindow` to allow upgrades at any time.
- If `timeZone` is not specified, it defaults to UTC.

## Maintenance Mode

PeerDB has a built-in maintenance mode that gracefully pauses all running mirrors before an upgrade and resumes them after. The operator integrates this via Kubernetes Jobs:

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBCluster
metadata:
  name: peerdb
spec:
  version: "v0.37.0"
  maintenance: {}
  # ... rest of spec
```

When `spec.maintenance` is set, the upgrade flow becomes:

1. **StartMaintenance** — A Job runs using the `flow-maintenance` image with `start` command. This triggers PeerDB's `StartMaintenance` Temporal workflow, which waits for running snapshots, enables maintenance mode (`PEERDB_MAINTENANCE_MODE_ENABLED`), and pauses all running mirrors.
2. **Normal upgrade** — Config, init jobs, Flow API, Server, and UI are rolled out in order.
3. **EndMaintenance** — A Job runs with the `end` command, resuming all previously paused mirrors and disabling maintenance mode.

While maintenance mode is active, mirrors cannot be created or mutated through PeerDB.

### Customizing the Maintenance Job

```yaml
spec:
  maintenance:
    image: "custom-registry/flow-maintenance:v1.0.0"  # Override image
    backoffLimit: 6                                     # Retry count
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
```

If a maintenance Job fails, the operator deletes it and retries automatically. A `Degraded` condition is set so you can monitor failures via:

```bash
kubectl get peerdbcluster <name> -o jsonpath='{.status.conditions}' | jq '.[] | select(.type=="MaintenanceMode")'
```

## Monitoring Upgrade Progress

### Quick Status

```bash
kubectl get peerdbcluster <name> -o wide
```

The `Upgrade` column (priority column) shows the current phase.

### Detailed Upgrade Status

```bash
kubectl get peerdbcluster <name> -o jsonpath='{.status.upgrade}' | jq .
```

Example output:

```json
{
  "fromVersion": "v0.36.7",
  "toVersion": "v0.37.0",
  "phase": "FlowAPI",
  "startedAt": "2026-03-04T02:15:00Z",
  "message": "Upgrading Flow API deployment"
}
```

### Upgrade Phases

| Phase | Description |
|-------|-------------|
| `Waiting` | Version change detected; waiting for maintenance window or manual approval |
| `Config` | Updating ConfigMap and Secret references |
| `InitJobs` | Re-running init jobs (if needed for the new version) |
| `FlowAPI` | Rolling out Flow API Deployment |
| `PeerDBServer` | Rolling out PeerDB Server Deployment |
| `UI` | Rolling out UI Deployment |
| `EndMaintenance` | Running EndMaintenance Job (resuming mirrors) |
| `Complete` | Upgrade finished successfully |
| `Blocked` | Upgrade blocked — dependencies are unhealthy |
| `StartMaintenance` | Running StartMaintenance Job (pausing mirrors) |

### Watch Upgrade Events

```bash
kubectl get events --field-selector involvedObject.name=<cluster-name> --sort-by='.lastTimestamp'
```

### Blocked Upgrade

If the upgrade phase shows `Blocked`, it means one or more dependencies are unhealthy and must be fixed before the upgrade can proceed:

```bash
# Check what's blocking
kubectl get peerdbcluster <name> -o jsonpath='{.status.conditions}' | jq '.[] | select(.type=="UpgradeInProgress")'
```

Common blockers:
- Catalog database unreachable
- Temporal service unhealthy
- Failed init jobs from a previous version

Fix the underlying issue and the upgrade will resume automatically.

## Rollback

To rollback, change `spec.version` back to the previous version:

```bash
kubectl patch peerdbcluster <name> --type merge -p '{"spec":{"version":"v0.36.7"}}'
```

This triggers a new "upgrade" to the old version, following the same ordered rollout process. There is no special rollback mechanism — a version change in either direction is treated identically.

## Pausing Reconciliation

To stop all changes (including an in-progress upgrade):

```bash
kubectl patch peerdbcluster <name> --type merge -p '{"spec":{"paused":true}}'
```

While paused:
- The controller will not make any changes to managed resources.
- The `Ready` condition will show reason `Paused`.
- Existing pods continue running — pausing does not disrupt traffic.

To resume:

```bash
kubectl patch peerdbcluster <name> --type merge -p '{"spec":{"paused":false}}'
```

## Worker Pool Upgrades

Worker and snapshot pools inherit their image from the referenced PeerDBCluster (unless overridden with `spec.image`). When the cluster version changes:

- Pools **without** an explicit `spec.image` will automatically pick up the new version on their next reconciliation.
- Pools **with** an explicit `spec.image` must be updated independently:
  ```bash
  kubectl patch peerdbworkerpool <name> --type merge -p '{"spec":{"image":"ghcr.io/peerdb-io/flow-worker:v0.37.0"}}'
  ```
