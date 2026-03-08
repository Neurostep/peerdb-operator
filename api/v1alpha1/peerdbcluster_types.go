/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition type constants for PeerDBCluster status.
const (
	// ConditionReady indicates the overall readiness of the PeerDBCluster.
	ConditionReady = "Ready"
	// ConditionCatalogReady indicates the catalog (PostgreSQL) connection is healthy.
	ConditionCatalogReady = "CatalogReady"
	// ConditionTemporalReady indicates the Temporal connection is healthy.
	ConditionTemporalReady = "TemporalReady"
	// ConditionInitialized indicates all init jobs have completed successfully.
	ConditionInitialized = "Initialized"
	// ConditionComponentsReady indicates all managed components are ready.
	ConditionComponentsReady = "ComponentsReady"
	// ConditionReconciling indicates the controller is actively reconciling.
	ConditionReconciling = "Reconciling"
	// ConditionDegraded indicates the cluster is running but in a degraded state.
	ConditionDegraded = "Degraded"
	// ConditionUpgradeInProgress indicates a version upgrade is in progress.
	ConditionUpgradeInProgress = "UpgradeInProgress"
	// ConditionBackupSafe indicates whether it is safe to take a backup (no rolling restarts or upgrades in progress).
	ConditionBackupSafe = "BackupSafe"
	// ConditionMaintenanceMode indicates PeerDB maintenance mode is active.
	ConditionMaintenanceMode = "MaintenanceMode"
)

// Reason constants for status conditions.
const (
	// ReasonPaused indicates reconciliation is paused.
	ReasonPaused = "Paused"
	// ReasonReconciling indicates active reconciliation.
	ReasonReconciling = "Reconciling"
	// ReasonReconcileComplete indicates reconciliation completed successfully.
	ReasonReconcileComplete = "ReconcileComplete"
	// ReasonClusterReady indicates the cluster is fully ready.
	ReasonClusterReady = "ClusterReady"
	// ReasonClusterNotReady indicates the cluster is not fully ready.
	ReasonClusterNotReady = "ClusterNotReady"
	// ReasonDependencyNotReady indicates a dependency is not ready.
	ReasonDependencyNotReady = "DependencyNotReady"
	// ReasonSecretNotFound indicates a required secret was not found.
	ReasonSecretNotFound = "SecretNotFound"
	// ReasonSecretFound indicates a required secret is available.
	ReasonSecretFound = "SecretFound"
	// ReasonConfigured indicates a dependency is configured.
	ReasonConfigured = "Configured"
	// ReasonJobsCompleted indicates all init jobs completed.
	ReasonJobsCompleted = "JobsCompleted"
	// ReasonJobsPending indicates init jobs have not completed.
	ReasonJobsPending = "JobsPending"
	// ReasonJobFailed indicates an init job has failed.
	ReasonJobFailed = "JobFailed"
	// ReasonAllReady indicates all components are ready.
	ReasonAllReady = "AllReady"
	// ReasonComponentsNotReady indicates some components are not ready.
	ReasonComponentsNotReady = "ComponentsNotReady"
	// ReasonDeploymentCreated indicates a deployment was just created.
	ReasonDeploymentCreated = "DeploymentCreated"
	// ReasonDeploymentReady indicates a deployment is ready.
	ReasonDeploymentReady = "DeploymentReady"
	// ReasonDeploymentNotReady indicates a deployment is not ready.
	ReasonDeploymentNotReady = "DeploymentNotReady"
	// ReasonStatefulSetCreated indicates a statefulset was just created.
	ReasonStatefulSetCreated = "StatefulSetCreated"
	// ReasonStatefulSetReady indicates a statefulset is ready.
	ReasonStatefulSetReady = "StatefulSetReady"
	// ReasonStatefulSetNotReady indicates a statefulset is not ready.
	ReasonStatefulSetNotReady = "StatefulSetNotReady"
	// ReasonClusterNotFound indicates the referenced cluster was not found.
	ReasonClusterNotFound = "ClusterNotFound"
	// ReasonUpgradeInProgress indicates a version upgrade is in progress.
	ReasonUpgradeInProgress = "UpgradeInProgress"
	// ReasonUpgradeComplete indicates a version upgrade is complete.
	ReasonUpgradeComplete = "UpgradeComplete"
	// ReasonDegraded indicates the cluster is degraded.
	ReasonDegraded = "Degraded"
	// ReasonUpgradeBlocked indicates an upgrade is blocked by unhealthy dependencies.
	ReasonUpgradeBlocked = "UpgradeBlocked"
	// ReasonMaintenanceWindow indicates an upgrade is waiting for the maintenance window.
	ReasonMaintenanceWindow = "MaintenanceWindow"
	// ReasonVersionSkew indicates a version skew between components.
	ReasonVersionSkew = "VersionSkew"
	// ReasonBackupInProgress indicates a backup is in progress and destructive operations are fenced.
	ReasonBackupInProgress = "BackupInProgress"
	// ReasonBackupSafe indicates the cluster is safe for backup.
	ReasonBackupSafe = "BackupSafe"
	// ReasonBackupUnsafe indicates the cluster is not safe for backup (upgrade or rollout in progress).
	ReasonBackupUnsafe = "BackupUnsafe"
	// ReasonMaintenanceStarting indicates maintenance mode is being activated.
	ReasonMaintenanceStarting = "MaintenanceStarting"
	// ReasonMaintenanceActive indicates maintenance mode is active.
	ReasonMaintenanceActive = "MaintenanceActive"
	// ReasonMaintenanceEnding indicates maintenance mode is being deactivated.
	ReasonMaintenanceEnding = "MaintenanceEnding"
	// ReasonMaintenanceComplete indicates maintenance mode has been deactivated.
	ReasonMaintenanceComplete = "MaintenanceComplete"
	// ReasonMaintenanceFailed indicates a maintenance mode job failed.
	ReasonMaintenanceFailed = "MaintenanceFailed"
)

// Annotation constants for PeerDBCluster.
const (
	// AnnotationBackupInProgress is set on a PeerDBCluster to fence destructive
	// reconciliation (rolling restarts, upgrades, scale changes) while a backup
	// is in progress. The operator continues health monitoring and status updates
	// but skips any mutations to managed Deployments, StatefulSets, and Jobs.
	// Set to any non-empty value (e.g., "true") to activate fencing.
	AnnotationBackupInProgress = "peerdb.io/backup-in-progress"
)

// SecretKeySelector references a specific key within a Kubernetes Secret.
type SecretKeySelector struct {
	// name is the name of the Secret.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// key is the key within the Secret.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ServiceSpec defines how a component's Service is exposed.
type ServiceSpec struct {
	// type is the Kubernetes Service type.
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer
	// +kubebuilder:default="ClusterIP"
	// +optional
	Type string `json:"type,omitempty"`
	// port is the port the Service listens on.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port *int32 `json:"port,omitempty"`
	// annotations are additional annotations to apply to the Service.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ServiceAccountConfig configures the ServiceAccount used by PeerDB components.
type ServiceAccountConfig struct {
	// create indicates whether the operator should create a ServiceAccount.
	// +kubebuilder:default=true
	// +optional
	Create bool `json:"create,omitempty"`
	// annotations are additional annotations to apply to the ServiceAccount.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// CatalogSpec defines the connection to the PostgreSQL catalog database.
type CatalogSpec struct {
	// host is the hostname of the PostgreSQL server.
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// port is the port of the PostgreSQL server.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=5432
	// +optional
	Port int32 `json:"port,omitempty"`
	// database is the name of the catalog database.
	// +kubebuilder:validation:MinLength=1
	Database string `json:"database"`
	// user is the PostgreSQL user for catalog access.
	// +kubebuilder:validation:MinLength=1
	User string `json:"user"`
	// passwordSecretRef references the Secret containing the PostgreSQL password.
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef"`
	// sslMode is the PostgreSQL SSL connection mode.
	// +kubebuilder:validation:Enum=disable;require;verify-ca;verify-full
	// +kubebuilder:default="require"
	// +optional
	SSLMode string `json:"sslMode,omitempty"`
}

// TemporalSpec defines the connection to the Temporal server.
type TemporalSpec struct {
	// address is the host:port of the Temporal frontend service.
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`
	// namespace is the Temporal namespace used by PeerDB.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// tlsSecretRef optionally references a Secret containing TLS credentials for Temporal.
	// The Secret should contain "tls.crt" and "tls.key" entries.
	// +optional
	TLSSecretRef *SecretKeySelector `json:"tlsSecretRef,omitempty"`
}

// DependenciesSpec defines external dependencies required by PeerDB.
type DependenciesSpec struct {
	// catalog is the PostgreSQL catalog database connection configuration.
	Catalog CatalogSpec `json:"catalog"`
	// temporal is the Temporal server connection configuration.
	Temporal TemporalSpec `json:"temporal"`
}

// FlowAPISpec defines the configuration for the Flow API component.
type FlowAPISpec struct {
	// image overrides the default Flow API container image.
	// +optional
	Image *string `json:"image,omitempty"`
	// replicas is the number of Flow API replicas.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// resources defines compute resource requirements for the Flow API container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// service configures the Flow API Kubernetes Service.
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`
}

// PeerDBServerSpec defines the configuration for the PeerDB Server (PostgreSQL-compatible query endpoint).
type PeerDBServerSpec struct {
	// image overrides the default PeerDB Server container image.
	// +optional
	Image *string `json:"image,omitempty"`
	// replicas is the number of PeerDB Server replicas.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// resources defines compute resource requirements for the PeerDB Server container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// service configures the PeerDB Server Kubernetes Service.
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`
	// passwordSecretRef references the Secret containing the PeerDB Server password.
	// +optional
	PasswordSecretRef *SecretKeySelector `json:"passwordSecretRef,omitempty"`
}

// UISpec defines the configuration for the PeerDB UI component.
type UISpec struct {
	// image overrides the default PeerDB UI container image.
	// +optional
	Image *string `json:"image,omitempty"`
	// replicas is the number of PeerDB UI replicas.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// resources defines compute resource requirements for the PeerDB UI container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// service configures the PeerDB UI Kubernetes Service.
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`
	// passwordSecretRef references the Secret containing the UI login password.
	// +optional
	PasswordSecretRef *SecretKeySelector `json:"passwordSecretRef,omitempty"`
	// nextAuthSecretRef references the Secret containing the NextAuth secret for session signing.
	// +optional
	NextAuthSecretRef *SecretKeySelector `json:"nextAuthSecretRef,omitempty"`
	// nextAuthURL is the canonical URL for NextAuth.js (e.g., "http://localhost:3000").
	// +kubebuilder:default="http://localhost:3000"
	// +optional
	NextAuthURL *string `json:"nextAuthURL,omitempty"`
}

// AuthProxyCredentials holds the credentials for the authentication proxy.
type AuthProxyCredentials struct {
	// username is the auth proxy username.
	// +kubebuilder:validation:MinLength=1
	Username string `json:"username"`
	// passwordSecretRef references the Secret containing the auth proxy password.
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef"`
}

// AuthProxySpec defines the configuration for the optional authentication proxy.
type AuthProxySpec struct {
	// image overrides the default Auth Proxy container image.
	// +optional
	Image *string `json:"image,omitempty"`
	// replicas is the number of Auth Proxy replicas.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// resources defines compute resource requirements for the Auth Proxy container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// service configures the Auth Proxy Kubernetes Service.
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`
	// credentials configures the authentication credentials for the proxy.
	Credentials AuthProxyCredentials `json:"credentials"`
}

// ComponentsSpec defines the PeerDB control plane components.
type ComponentsSpec struct {
	// flowAPI configures the Flow API component.
	// +optional
	FlowAPI *FlowAPISpec `json:"flowAPI,omitempty"`
	// peerDBServer configures the PeerDB Server component.
	// +optional
	PeerDBServer *PeerDBServerSpec `json:"peerDBServer,omitempty"`
	// ui configures the PeerDB UI component.
	// +optional
	UI *UISpec `json:"ui,omitempty"`
	// authProxy configures the optional authentication proxy.
	// +optional
	AuthProxy *AuthProxySpec `json:"authProxy,omitempty"`
}

// InitJobSpec defines the configuration for an init job.
type InitJobSpec struct {
	// enabled controls whether this init job runs.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// image overrides the default container image for the init job.
	// +optional
	Image *string `json:"image,omitempty"`
	// backoffLimit is the number of retries before marking the job as failed.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=4
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
	// resources defines compute resource requirements for the init job container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// InitSpec defines the init jobs that run during cluster setup.
type InitSpec struct {
	// temporalNamespaceRegistration configures the job that registers the Temporal namespace.
	// +optional
	TemporalNamespaceRegistration *InitJobSpec `json:"temporalNamespaceRegistration,omitempty"`
	// temporalSearchAttributes configures the job that registers Temporal search attributes.
	// +optional
	TemporalSearchAttributes *InitJobSpec `json:"temporalSearchAttributes,omitempty"`
}

// MaintenanceSpec configures PeerDB maintenance mode for upgrades.
// When enabled, the operator triggers PeerDB's maintenance workflows
// to gracefully pause mirrors before upgrading and resume them after.
type MaintenanceSpec struct {
	// image overrides the default flow-maintenance container image.
	// +optional
	Image *string `json:"image,omitempty"`
	// backoffLimit is the number of retries before marking the maintenance job as failed.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=4
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
	// resources defines compute resource requirements for the maintenance job container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// UpgradePolicy controls whether the operator automatically performs version upgrades.
// +kubebuilder:validation:Enum=Automatic;Manual
type UpgradePolicy string

const (
	// UpgradePolicyAutomatic allows the operator to perform upgrades immediately.
	UpgradePolicyAutomatic UpgradePolicy = "Automatic"
	// UpgradePolicyManual requires the user to set policy to Automatic to proceed.
	UpgradePolicyManual UpgradePolicy = "Manual"
)

// MaintenanceWindow defines a daily time window during which upgrades may start.
type MaintenanceWindow struct {
	// start is the beginning of the maintenance window in HH:MM (24h) format.
	// +kubebuilder:validation:Pattern=`^([01]\d|2[0-3]):[0-5]\d$`
	Start string `json:"start"`
	// end is the end of the maintenance window in HH:MM (24h) format.
	// +kubebuilder:validation:Pattern=`^([01]\d|2[0-3]):[0-5]\d$`
	End string `json:"end"`
	// timeZone is an IANA timezone name (e.g. "UTC", "America/Los_Angeles").
	// Defaults to UTC if not specified.
	// +optional
	TimeZone *string `json:"timeZone,omitempty"`
}

// UpgradePhase represents the current phase of a version upgrade.
type UpgradePhase string

const (
	UpgradePhaseComplete         UpgradePhase = "Complete"
	UpgradePhaseWaiting          UpgradePhase = "Waiting"
	UpgradePhaseBlocked          UpgradePhase = "Blocked"
	UpgradePhaseStartMaintenance UpgradePhase = "StartMaintenance"
	UpgradePhaseConfig           UpgradePhase = "Config"
	UpgradePhaseInitJobs         UpgradePhase = "InitJobs"
	UpgradePhaseFlowAPI          UpgradePhase = "FlowAPI"
	UpgradePhaseServer           UpgradePhase = "PeerDBServer"
	UpgradePhaseUI               UpgradePhase = "UI"
	UpgradePhaseEndMaintenance   UpgradePhase = "EndMaintenance"
)

// UpgradeStatus tracks the progress of a version upgrade.
type UpgradeStatus struct {
	// fromVersion is the version the cluster is upgrading from.
	// +optional
	FromVersion string `json:"fromVersion,omitempty"`
	// toVersion is the version the cluster is upgrading to.
	// +optional
	ToVersion string `json:"toVersion,omitempty"`
	// phase is the current phase of the upgrade.
	// +optional
	Phase UpgradePhase `json:"phase,omitempty"`
	// startedAt is the time the upgrade was started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// message is a human-readable description of the current upgrade state.
	// +optional
	Message string `json:"message,omitempty"`
}

// EndpointStatus reports the addresses of PeerDB endpoints.
type EndpointStatus struct {
	// serverAddress is the address of the PeerDB Server endpoint.
	// +optional
	ServerAddress string `json:"serverAddress,omitempty"`
	// uiAddress is the address of the PeerDB UI endpoint.
	// +optional
	UIAddress string `json:"uiAddress,omitempty"`
	// flowAPIAddress is the address of the Flow API endpoint.
	// +optional
	FlowAPIAddress string `json:"flowAPIAddress,omitempty"`
}

// PeerDBClusterSpec defines the desired state of PeerDBCluster.
type PeerDBClusterSpec struct {
	// version is the PeerDB image tag to deploy (e.g. "v0.36.7").
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
	// imagePullSecrets is a list of references to Secrets for pulling container images.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// serviceAccount configures the ServiceAccount used by PeerDB components.
	// +optional
	ServiceAccount *ServiceAccountConfig `json:"serviceAccount,omitempty"`
	// dependencies defines external systems required by PeerDB.
	Dependencies DependenciesSpec `json:"dependencies"`
	// components configures the PeerDB control plane components.
	// +optional
	Components *ComponentsSpec `json:"components,omitempty"`
	// init configures the init jobs that run during cluster setup.
	// +optional
	Init *InitSpec `json:"init,omitempty"`
	// paused stops reconciliation of this PeerDBCluster when set to true.
	// +optional
	Paused bool `json:"paused,omitempty"`
	// upgradePolicy controls whether upgrades are applied automatically or require manual approval.
	// +kubebuilder:validation:Enum=Automatic;Manual
	// +kubebuilder:default="Automatic"
	// +optional
	UpgradePolicy *UpgradePolicy `json:"upgradePolicy,omitempty"`
	// maintenanceWindow defines a daily time window during which upgrades may start.
	// Only used when upgradePolicy is Automatic.
	// +optional
	MaintenanceWindow *MaintenanceWindow `json:"maintenanceWindow,omitempty"`
	// maintenance configures PeerDB maintenance mode for graceful upgrades.
	// When configured, the operator runs maintenance workflows to pause mirrors
	// before upgrading and resume them after.
	// +optional
	Maintenance *MaintenanceSpec `json:"maintenance,omitempty"`
}

// PeerDBClusterStatus defines the observed state of PeerDBCluster.
type PeerDBClusterStatus struct {
	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// conditions represent the current state of the PeerDBCluster.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// endpoints reports the addresses of PeerDB endpoints.
	// +optional
	Endpoints *EndpointStatus `json:"endpoints,omitempty"`
	// upgrade tracks the progress of a version upgrade.
	// +optional
	Upgrade *UpgradeStatus `json:"upgrade,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Upgrade",type="string",JSONPath=`.status.upgrade.phase`,priority=1

// PeerDBCluster is the Schema for the peerdbclusters API.
// It manages PeerDB's control plane components including Flow API, PeerDB Server, and PeerDB UI.
type PeerDBCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PeerDBCluster
	// +required
	Spec PeerDBClusterSpec `json:"spec"`

	// status defines the observed state of PeerDBCluster
	// +optional
	Status PeerDBClusterStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// PeerDBClusterList contains a list of PeerDBCluster.
type PeerDBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PeerDBCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PeerDBCluster{}, &PeerDBClusterList{})
}
