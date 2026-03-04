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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SnapshotPoolStorageSpec defines the persistent volume claim configuration for snapshot workers.
type SnapshotPoolStorageSpec struct {
	// size is the requested storage size for each snapshot worker PVC.
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`

	// storageClassName is the name of the StorageClass to use for the PVCs.
	// If not set, the default StorageClass is used.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// PeerDBSnapshotPoolSpec defines the desired state of PeerDBSnapshotPool.
type PeerDBSnapshotPoolSpec struct {
	// clusterRef is the name of the PeerDBCluster resource in the same namespace
	// that this snapshot pool belongs to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ClusterRef string `json:"clusterRef"`

	// image is the container image to use for snapshot workers.
	// If not set, the image is inherited from the referenced PeerDBCluster.
	// +optional
	Image string `json:"image,omitempty"`

	// replicas is the desired number of snapshot worker replicas.
	// Snapshot workers are bursty — scale up during initial loads and scale to 0 when idle.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// resources defines the compute resource requirements for each snapshot worker.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// storage defines the persistent volume claim configuration for snapshot workers.
	// +kubebuilder:validation:Required
	Storage SnapshotPoolStorageSpec `json:"storage"`

	// terminationGracePeriodSeconds is the duration in seconds to wait before forcefully
	// terminating a snapshot worker pod. Should be set high enough to allow long-running
	// snapshot operations to complete gracefully.
	// +kubebuilder:default=600
	// +kubebuilder:validation:Minimum=0
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// nodeSelector is a map of node labels for pod scheduling constraints.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// tolerations are tolerations for pod scheduling.
	// +listType=atomic
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// affinity defines scheduling constraints for snapshot worker pods.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// extraEnv is a list of additional environment variables to set on snapshot worker containers.
	// +listType=map
	// +listMapKey=name
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// podAnnotations are additional annotations to apply to snapshot worker pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// podLabels are additional labels to apply to snapshot worker pods.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
}

// PeerDBSnapshotPoolStatus defines the observed state of PeerDBSnapshotPool.
type PeerDBSnapshotPoolStatus struct {
	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// replicas is the total number of snapshot worker pods managed by this pool.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// readyReplicas is the number of snapshot worker pods that are ready.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// conditions represent the current state of the PeerDBSnapshotPool resource.
	//
	// Standard condition types:
	// - "Ready": all desired replicas are ready and the pool is operational
	// - "Available": at least one snapshot worker is available to accept work
	//
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Whether the snapshot pool is ready"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=`.status.replicas`,description="Total number of snapshot worker replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// PeerDBSnapshotPool is the Schema for the peerdbsnapshotpools API.
// It manages a pool of snapshot worker StatefulSets that handle initial data loads.
// Snapshot workers are bursty — they scale up during initial loads and can scale to 0 when idle.
type PeerDBSnapshotPool struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PeerDBSnapshotPool
	// +required
	Spec PeerDBSnapshotPoolSpec `json:"spec"`

	// status defines the observed state of PeerDBSnapshotPool
	// +optional
	Status PeerDBSnapshotPoolStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// PeerDBSnapshotPoolList contains a list of PeerDBSnapshotPool
type PeerDBSnapshotPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PeerDBSnapshotPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PeerDBSnapshotPool{}, &PeerDBSnapshotPoolList{})
}
