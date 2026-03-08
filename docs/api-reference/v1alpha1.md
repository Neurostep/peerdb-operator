# API Reference — v1alpha1

API group: `peerdb.peerdb.io/v1alpha1`

This document describes all Custom Resource Definitions (CRDs) managed by the PeerDB Operator.

---

## PeerDBCluster

`PeerDBCluster` represents the PeerDB control plane: Flow API, PeerDB Server, UI, optional auth proxy, and shared configuration. It also manages init jobs for Temporal namespace registration and search attribute setup.

### Print Columns

| Name | JSON Path | Type | Priority | Description |
|------|-----------|------|----------|-------------|
| Ready | `.status.conditions[?(@.type=="Ready")].status` | string | 0 | Whether the cluster is fully ready |
| Version | `.spec.version` | string | 0 | PeerDB version |
| Age | `.metadata.creationTimestamp` | date | 0 | Time since creation |
| Upgrade | `.status.upgrade.phase` | string | 1 | Current upgrade phase (if any) |

### PeerDBClusterSpec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `version` | `string` | **Yes** | — | PeerDB version to deploy. Must be non-empty (minLength: 1). |
| `imagePullSecrets` | `[]LocalObjectReference` | No | `[]` | List of image pull secrets for pulling PeerDB container images. |
| `serviceAccount` | [`ServiceAccountConfig`](#serviceaccountconfig) | No | `{create: true}` | Service account configuration for PeerDB pods. |
| `dependencies` | [`DependenciesSpec`](#dependenciesspec) | **Yes** | — | External dependency connection configuration (catalog DB, Temporal). |
| `components` | [`ComponentsSpec`](#componentsspec) | No | See sub-fields | Configuration for individual PeerDB components. |
| `init` | [`InitSpec`](#initspec) | No | See sub-fields | Init job configuration for Temporal setup. |
| `paused` | `bool` | No | `false` | When true, the operator stops reconciling this cluster. |
| `upgradePolicy` | [`UpgradePolicy`](#upgradepolicy) | No | `Automatic` | Controls how version upgrades are applied. Enum: `Automatic`, `Manual`. |
| `maintenanceWindow` | [`MaintenanceWindow`](#maintenancewindow) | No | — | Time window for automatic upgrades. Only used when `upgradePolicy` is `Automatic`. |
| `maintenance` | [`MaintenanceSpec`](#maintenancespec) | No | — | Configures PeerDB maintenance mode for graceful upgrades. When set, the operator pauses mirrors before upgrading and resumes them after. |

### PeerDBClusterStatus

| Field | Type | Description |
|-------|------|-------------|
| `observedGeneration` | `int64` | The generation most recently observed by the controller. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions. See [Condition Types](#condition-types). |
| `endpoints` | [`EndpointStatus`](#endpointstatus) | Discovered service endpoints for PeerDB components. |
| `upgrade` | [`UpgradeStatus`](#upgradestatus) | Current upgrade state, if an upgrade is in progress. |

---

### DependenciesSpec

Container for external dependency configuration.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `catalog` | [`CatalogSpec`](#catalogspec) | **Yes** | — | Connection details for the PostgreSQL catalog database. |
| `temporal` | [`TemporalSpec`](#temporalspec) | **Yes** | — | Connection details for the Temporal server. |

### CatalogSpec

Connection configuration for the external PostgreSQL catalog database.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | `string` | **Yes** | — | Hostname of the PostgreSQL server. MinLength: 1. |
| `port` | `int32` | No | `5432` | Port number (1–65535). |
| `database` | `string` | **Yes** | — | Database name. |
| `user` | `string` | **Yes** | — | Database user. |
| `passwordSecretRef` | [`SecretKeySelector`](#secretkeyselector) | **Yes** | — | Reference to a Secret key containing the database password. |
| `sslMode` | `string` | No | `require` | PostgreSQL SSL mode. Enum: `disable`, `require`, `verify-ca`, `verify-full`. |

### TemporalSpec

Connection configuration for the external Temporal server.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `address` | `string` | **Yes** | — | Temporal server address in `host:port` format. |
| `namespace` | `string` | **Yes** | — | Temporal namespace to use. |
| `tlsSecretRef` | [`SecretKeySelector`](#secretkeyselector) | No | — | Reference to a TLS Secret. The Secret should contain `tls.crt` and `tls.key` entries. |

### SecretKeySelector

Reference to a specific key within a Kubernetes Secret.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | **Yes** | — | Name of the Secret. |
| `key` | `string` | **Yes** | — | Key within the Secret. |

### ServiceAccountConfig

Configuration for the ServiceAccount used by PeerDB pods.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `create` | `bool` | No | `true` | Whether the operator should create a ServiceAccount. |
| `annotations` | `map[string]string` | No | `{}` | Annotations to add to the ServiceAccount (e.g., for IAM role binding). |

### ComponentsSpec

Configuration for individual PeerDB control-plane components.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `flowAPI` | [`FlowAPISpec`](#flowapispec) | No | See sub-fields | Flow API deployment configuration. |
| `peerDBServer` | [`PeerDBServerSpec`](#peerdbserverspec) | No | See sub-fields | PeerDB Server deployment configuration. |
| `ui` | [`UISpec`](#uispec) | No | See sub-fields | PeerDB UI deployment configuration. |
| `authProxy` | [`AuthProxySpec`](#authproxyspec) | No | — | Auth proxy deployment configuration. Not deployed unless specified. |

### FlowAPISpec

Configuration for the Flow API component.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | `*string` | No | `ghcr.io/peerdb-io/flow-api` | Container image for the Flow API. |
| `replicas` | `*int32` | No | `1` | Number of replicas (min: 0). |
| `resources` | `*ResourceRequirements` | No | — | CPU/memory resource requests and limits. |
| `service` | [`ServiceSpec`](#servicespec) | No | See defaults | Service configuration. Default ports: 8112 (gRPC), 8113 (HTTP). |

### PeerDBServerSpec

Configuration for the PeerDB Server component (Postgres wire-protocol proxy).

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | `*string` | No | `ghcr.io/peerdb-io/peerdb-server` | Container image for PeerDB Server. |
| `replicas` | `*int32` | No | `1` | Number of replicas (min: 0). |
| `resources` | `*ResourceRequirements` | No | — | CPU/memory resource requests and limits. |
| `service` | [`ServiceSpec`](#servicespec) | No | See defaults | Service configuration. Default port: 9900. |
| `passwordSecretRef` | [`SecretKeySelector`](#secretkeyselector) | No | — | Reference to a Secret key containing the PeerDB Server password. |

### UISpec

Configuration for the PeerDB UI component.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | `*string` | No | `ghcr.io/peerdb-io/peerdb-ui` | Container image for PeerDB UI. |
| `replicas` | `*int32` | No | `1` | Number of replicas (min: 0). |
| `resources` | `*ResourceRequirements` | No | — | CPU/memory resource requests and limits. |
| `service` | [`ServiceSpec`](#servicespec) | No | See defaults | Service configuration. Default port: 3000. |
| `passwordSecretRef` | [`SecretKeySelector`](#secretkeyselector) | No | — | Reference to a Secret key containing the UI password. |
| `nextAuthSecretRef` | [`SecretKeySelector`](#secretkeyselector) | No | — | Reference to a Secret key containing the NextAuth secret. |
| `nextAuthURL` | `*string` | No | `http://localhost:3000` | The NextAuth URL for the UI. |

### AuthProxySpec

Configuration for the optional authentication proxy component.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | `*string` | No | — | Container image for the auth proxy. |
| `replicas` | `*int32` | No | `1` | Number of replicas (min: 0). |
| `resources` | `*ResourceRequirements` | No | — | CPU/memory resource requests and limits. |
| `service` | [`ServiceSpec`](#servicespec) | No | See defaults | Service configuration. |
| `credentials` | [`AuthProxyCredentials`](#authproxycredentials) | **Yes** | — | Authentication credentials for the proxy. |

### AuthProxyCredentials

Credentials for the auth proxy.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `username` | `string` | **Yes** | — | Username for the auth proxy. |
| `passwordSecretRef` | [`SecretKeySelector`](#secretkeyselector) | **Yes** | — | Reference to a Secret key containing the auth proxy password. |

### ServiceSpec

Kubernetes Service configuration used by multiple components.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | `string` | No | `ClusterIP` | Service type. Enum: `ClusterIP`, `LoadBalancer`. |
| `port` | `*int32` | No | Component-specific | Service port (1–65535). |
| `annotations` | `map[string]string` | No | `{}` | Annotations to add to the Service. |

### InitSpec

Configuration for one-time initialization jobs.

> **Note:** Init jobs are create-once — they are not re-run on spec changes. Changing Temporal config after initial setup requires manual job deletion.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `temporalNamespaceRegistration` | [`InitJobSpec`](#initjobspec) | No | `{enabled: true}` | Job that registers the Temporal namespace. |
| `temporalSearchAttributes` | [`InitJobSpec`](#initjobspec) | No | `{enabled: true}` | Job that creates PeerDB search attributes in Temporal. |

### InitJobSpec

Configuration for an individual init job.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | `*bool` | No | `true` | Whether the init job should run. |
| `image` | `*string` | No | `temporalio/admin-tools` | Container image for the init job. |
| `backoffLimit` | `*int32` | No | `4` | Number of retries before marking the job as failed (min: 0). |
| `resources` | `*ResourceRequirements` | No | — | CPU/memory resource requests and limits. |

### MaintenanceWindow

Defines a time window during which automatic upgrades may be applied.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `start` | `string` | **Yes** | — | Start time in 24-hour `HH:MM` format. |
| `end` | `string` | **Yes** | — | End time in 24-hour `HH:MM` format. |
| `timeZone` | `*string` | No | `UTC` | IANA timezone name (e.g., `America/New_York`). |

### MaintenanceSpec

Configuration for PeerDB maintenance mode during upgrades. When configured, the operator runs maintenance Jobs (`ghcr.io/peerdb-io/flow-maintenance`) to gracefully pause all mirrors before upgrading and resume them after.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | `*string` | No | `ghcr.io/peerdb-io/flow-maintenance:stable-{version}` | Container image override for the maintenance Job. |
| `backoffLimit` | `*int32` | No | `4` | Number of retries before marking the maintenance Job as failed (min: 0). |
| `resources` | `*ResourceRequirements` | No | — | CPU/memory resource requests and limits for the maintenance Job container. |

### UpgradePolicy

`string` enum controlling how version upgrades are applied.

| Value | Description |
|-------|-------------|
| `Automatic` | The operator upgrades components automatically when `spec.version` changes. If a `maintenanceWindow` is set, upgrades are deferred to that window. |
| `Manual` | The operator sets the `UpgradeInProgress` condition but does not roll out changes until the user acknowledges. |

### EndpointStatus

Discovered service addresses for PeerDB components.

| Field | Type | Description |
|-------|------|-------------|
| `serverAddress` | `string` | Address of the PeerDB Server service (Postgres wire protocol, port 9900). |
| `uiAddress` | `string` | Address of the PeerDB UI service (HTTP, port 3000). |
| `flowAPIAddress` | `string` | Address of the Flow API service (gRPC 8112 / HTTP 8113). |

### UpgradeStatus

Tracks the state of a rolling version upgrade.

| Field | Type | Description |
|-------|------|-------------|
| `fromVersion` | `string` | The version being upgraded from. |
| `toVersion` | `string` | The version being upgraded to. |
| `phase` | `UpgradePhase` | Current upgrade phase. Values: `Complete`, `Waiting`, `Blocked`, `StartMaintenance`, `Config`, `InitJobs`, `FlowAPI`, `PeerDBServer`, `UI`, `EndMaintenance`. |
| `startedAt` | `*metav1.Time` | Timestamp when the upgrade started. |
| `message` | `string` | Human-readable message about the upgrade state. |

---

## PeerDBWorkerPool

`PeerDBWorkerPool` defines a pool of CDC Flow Workers deployed as a Kubernetes Deployment. Multiple pools can exist per cluster for different workload profiles. Worker pools are CPU/memory intensive (default: 2 CPU, 8Gi memory) and support independent horizontal autoscaling via HPA/KEDA.

Worker pools reference a `PeerDBCluster` by name (must be in the same namespace) to inherit connection configuration.

### Print Columns

| Name | JSON Path | Type | Priority | Description |
|------|-----------|------|----------|-------------|
| Ready | `.status.conditions[?(@.type=="Ready")].status` | string | 0 | Whether the worker pool is ready |
| Replicas | `.status.replicas` | integer | 0 | Current number of replicas |
| Age | `.metadata.creationTimestamp` | date | 0 | Time since creation |

### PeerDBWorkerPoolSpec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `clusterRef` | `string` | **Yes** | — | Name of the `PeerDBCluster` in the same namespace. |
| `image` | `string` | No | Inherited from cluster | Container image override. If unset, uses `ghcr.io/peerdb-io/flow-worker` with the cluster's version tag. |
| `replicas` | `*int32` | No | `2` | Desired number of replicas (min: 0). Ignored when `autoscaling.enabled` is `true`. |
| `resources` | `ResourceRequirements` | No | `requests: {cpu: "2", memory: "8Gi"}` | CPU/memory resource requests and limits. |
| `temporalTaskQueue` | `string` | No | Inherited from cluster | Temporal task queue name override. |
| `autoscaling` | [`AutoscalingSpec`](#autoscalingspec) | No | `{enabled: false}` | Horizontal autoscaling configuration. |
| `nodeSelector` | `map[string]string` | No | `{}` | Kubernetes node selector for pod scheduling. |
| `tolerations` | `[]Toleration` | No | `[]` | Kubernetes tolerations for pod scheduling. |
| `affinity` | `*Affinity` | No | — | Kubernetes affinity rules for pod scheduling. |
| `extraEnv` | `[]EnvVar` | No | `[]` | Additional environment variables to inject into worker pods. |
| `podAnnotations` | `map[string]string` | No | `{}` | Annotations to add to worker pods. |
| `podLabels` | `map[string]string` | No | `{}` | Labels to add to worker pods. |

### PeerDBWorkerPoolStatus

| Field | Type | Description |
|-------|------|-------------|
| `observedGeneration` | `int64` | The generation most recently observed by the controller. |
| `replicas` | `int32` | Total number of replicas managed by the Deployment. |
| `readyReplicas` | `int32` | Number of replicas that have passed readiness checks. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions. Types: `Ready`, `Available`. |

### AutoscalingSpec

Horizontal Pod Autoscaler configuration for worker pools.

> **Validation:** When `enabled` is `true`, `minReplicas` must be ≤ `maxReplicas`.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | `bool` | No | `false` | Whether to create an HPA for this pool. When `true`, the controller skips setting `spec.replicas` on the Deployment to avoid conflicts. |
| `minReplicas` | `*int32` | No | `1` | Minimum number of replicas (min: 1). |
| `maxReplicas` | `int32` | **Yes** (when enabled) | — | Maximum number of replicas (min: 1). |
| `targetCPUUtilization` | `*int32` | No | `70` | Target average CPU utilization percentage (1–100). |

---

## PeerDBSnapshotPool

`PeerDBSnapshotPool` defines a pool of Snapshot Workers deployed as a Kubernetes StatefulSet with persistent storage. Snapshot workers handle initial data loads and are typically bursty — scale up during snapshots, scale to zero when idle.

Snapshot pools reference a `PeerDBCluster` by name (must be in the same namespace) to inherit connection configuration.

> **Important:** StatefulSet `volumeClaimTemplates` are immutable. Changing storage settings (size, storage class) requires creating a new pool or manual migration.

### Print Columns

| Name | JSON Path | Type | Priority | Description |
|------|-----------|------|----------|-------------|
| Ready | `.status.conditions[?(@.type=="Ready")].status` | string | 0 | Whether the snapshot pool is ready |
| Replicas | `.status.replicas` | integer | 0 | Current number of replicas |
| Age | `.metadata.creationTimestamp` | date | 0 | Time since creation |

### PeerDBSnapshotPoolSpec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `clusterRef` | `string` | **Yes** | — | Name of the `PeerDBCluster` in the same namespace. |
| `image` | `string` | No | Inherited from cluster | Container image override. If unset, uses `ghcr.io/peerdb-io/flow-snapshot-worker` with the cluster's version tag. |
| `replicas` | `*int32` | No | `1` | Desired number of replicas (min: 0). |
| `resources` | `ResourceRequirements` | No | `requests: {cpu: "500m", memory: "1Gi"}` | CPU/memory resource requests and limits. |
| `storage` | [`SnapshotPoolStorageSpec`](#snapshotpoolstoragespec) | **Yes** | — | Persistent volume configuration. |
| `terminationGracePeriodSeconds` | `*int64` | No | `600` | Seconds to wait for graceful shutdown before force-killing (min: 0). |
| `nodeSelector` | `map[string]string` | No | `{}` | Kubernetes node selector for pod scheduling. |
| `tolerations` | `[]Toleration` | No | `[]` | Kubernetes tolerations for pod scheduling. |
| `affinity` | `*Affinity` | No | — | Kubernetes affinity rules for pod scheduling. |
| `extraEnv` | `[]EnvVar` | No | `[]` | Additional environment variables to inject into snapshot worker pods. |
| `podAnnotations` | `map[string]string` | No | `{}` | Annotations to add to snapshot worker pods. |
| `podLabels` | `map[string]string` | No | `{}` | Labels to add to snapshot worker pods. |

### PeerDBSnapshotPoolStatus

| Field | Type | Description |
|-------|------|-------------|
| `observedGeneration` | `int64` | The generation most recently observed by the controller. |
| `replicas` | `int32` | Total number of replicas managed by the StatefulSet. |
| `readyReplicas` | `int32` | Number of replicas that have passed readiness checks. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions. Types: `Ready`, `Available`. |

### SnapshotPoolStorageSpec

Persistent volume configuration for snapshot workers.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `size` | `resource.Quantity` | **Yes** | — | Size of the persistent volume claim (e.g., `10Gi`, `100Gi`). |
| `storageClassName` | `*string` | No | Cluster default | Kubernetes StorageClass name. If unset, the cluster's default StorageClass is used. |

---

## Condition Types

The following condition types are used in `PeerDBCluster` status:

| Condition Type | Description |
|----------------|-------------|
| `Ready` | Overall readiness of the cluster. `True` when all components are healthy and operational. |
| `CatalogReady` | Whether the catalog PostgreSQL database is reachable and configured. |
| `TemporalReady` | Whether the Temporal server is reachable and the namespace exists. |
| `Initialized` | Whether all init jobs (namespace registration, search attributes) have completed successfully. |
| `ComponentsReady` | Whether all PeerDB component Deployments (Flow API, Server, UI) have available replicas. |
| `Reconciling` | Set to `True` while the operator is actively reconciling changes (e.g., rolling out a new version). |
| `Degraded` | Set to `True` when one or more components are unhealthy but the cluster is partially operational. |
| `UpgradeInProgress` | Set to `True` when a version upgrade is in progress. |
| `BackupSafe` | Whether it is safe to take a backup. `True` when no upgrade or rolling restart is in progress. `False` with reason `BackupInProgress` when the `peerdb.io/backup-in-progress` annotation is set, or `BackupUnsafe` when an upgrade/rollout is active. |
| `MaintenanceMode` | Set to `True` when PeerDB maintenance mode is active (mirrors are paused for an upgrade). Set to `False` with reason `MaintenanceComplete` after mirrors are resumed. |

### Annotations

| Annotation | Description |
|------------|-------------|
| `peerdb.io/backup-in-progress` | Set to any non-empty value (e.g., `"true"`) to fence destructive reconciliation. The operator will continue health monitoring and status updates but skip mutations to Deployments, StatefulSets, and Jobs. Worker and snapshot pool controllers also respect this annotation on the referenced cluster. Remove the annotation to resume normal reconciliation. |

Worker and snapshot pool resources use the following condition types:

| Condition Type | Description |
|----------------|-------------|
| `Ready` | Overall readiness of the pool. `True` when the desired number of replicas are available. |
| `Available` | Whether at least one replica is available and serving traffic. |

## Upgrade Phases

The `UpgradeStatus.phase` field tracks progress through a rolling upgrade:

| Phase | Description |
|-------|-------------|
| `Waiting` | Upgrade is pending (e.g., waiting for a maintenance window). |
| `Blocked` | Upgrade is blocked (e.g., manual policy requires acknowledgement). |
| `StartMaintenance` | Running the StartMaintenance Job to pause mirrors before upgrade. |
| `Config` | Updating shared ConfigMap and configuration. |
| `InitJobs` | Re-running init jobs if needed. |
| `FlowAPI` | Rolling out the Flow API Deployment. |
| `PeerDBServer` | Rolling out the PeerDB Server Deployment. |
| `UI` | Rolling out the PeerDB UI Deployment. |
| `EndMaintenance` | Running the EndMaintenance Job to resume mirrors after upgrade. |
| `Complete` | Upgrade finished successfully. |
