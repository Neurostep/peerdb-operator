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

// AutoscalingSpec defines the autoscaling configuration for a PeerDBWorkerPool.
// +kubebuilder:validation:XValidation:rule="!self.enabled || !has(self.minReplicas) || self.minReplicas <= self.maxReplicas",message="minReplicas must be less than or equal to maxReplicas"
type AutoscalingSpec struct {
	// enabled controls whether horizontal pod autoscaling is active.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// minReplicas is the lower bound for the number of worker replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// maxReplicas is the upper bound for the number of worker replicas.
	// +kubebuilder:validation:Minimum=1
	// +required
	MaxReplicas int32 `json:"maxReplicas"`

	// targetCPUUtilization is the target average CPU utilization (percentage)
	// across all replicas used by the HPA to make scaling decisions.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=70
	// +optional
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`
}

// PeerDBWorkerPoolSpec defines the desired state of PeerDBWorkerPool.
type PeerDBWorkerPoolSpec struct {
	// clusterRef is the name of the PeerDBCluster resource in the same namespace
	// that this worker pool belongs to.
	// +kubebuilder:validation:MinLength=1
	// +required
	ClusterRef string `json:"clusterRef"`

	// image overrides the container image used for worker pods.
	// When omitted, the image is inherited from the referenced PeerDBCluster.
	// +optional
	Image string `json:"image,omitempty"`

	// replicas is the desired number of worker pods.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// resources defines the CPU and memory resource requirements for each worker pod.
	// Workers are CPU-heavy; the default requests are 2 CPU and 8Gi memory.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// temporalTaskQueue overrides the Temporal task queue name that workers poll.
	// When omitted, the task queue is inherited from the referenced PeerDBCluster.
	// +optional
	TemporalTaskQueue string `json:"temporalTaskQueue,omitempty"`

	// autoscaling configures horizontal pod autoscaling for this worker pool.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// nodeSelector constrains scheduling to nodes with matching labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// tolerations allow worker pods to be scheduled on tainted nodes.
	// +listType=atomic
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// affinity specifies scheduling constraints for worker pods,
	// useful for targeting IO or compute-optimized node pools.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// extraEnv defines additional environment variables injected into worker containers.
	// +listType=map
	// +listMapKey=name
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// podAnnotations are additional annotations applied to worker pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// podLabels are additional labels applied to worker pods.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
}

// PeerDBWorkerPoolStatus defines the observed state of PeerDBWorkerPool.
type PeerDBWorkerPoolStatus struct {
	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// replicas is the total number of worker pods managed by this pool.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// readyReplicas is the number of worker pods in the ready state.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// conditions represent the current state of the PeerDBWorkerPool resource.
	// Known condition types are "Ready" and "Available".
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Whether the worker pool is ready"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=`.status.replicas`,description="Total number of worker replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// PeerDBWorkerPool is the Schema for the peerdbworkerpools API.
// It manages CDC Flow Worker Deployments independently from the main PeerDBCluster,
// enabling independent scaling, HPA support, and multiple worker pools.
type PeerDBWorkerPool struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PeerDBWorkerPool
	// +required
	Spec PeerDBWorkerPoolSpec `json:"spec"`

	// status defines the observed state of PeerDBWorkerPool
	// +optional
	Status PeerDBWorkerPoolStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// PeerDBWorkerPoolList contains a list of PeerDBWorkerPool.
type PeerDBWorkerPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PeerDBWorkerPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PeerDBWorkerPool{}, &PeerDBWorkerPoolList{})
}
