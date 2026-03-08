package resources

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

// BuildStartMaintenanceJob creates a Job that triggers PeerDB's StartMaintenance workflow.
// This pauses all running mirrors and enables maintenance mode before an upgrade.
func BuildStartMaintenanceJob(cluster *v1alpha1.PeerDBCluster) *batchv1.Job {
	return buildMaintenanceJob(cluster, "start")
}

// BuildEndMaintenanceJob creates a Job that triggers PeerDB's EndMaintenance workflow.
// This resumes previously paused mirrors and disables maintenance mode after an upgrade.
func BuildEndMaintenanceJob(cluster *v1alpha1.PeerDBCluster) *batchv1.Job {
	return buildMaintenanceJob(cluster, "end")
}

func buildMaintenanceJob(cluster *v1alpha1.PeerDBCluster, action string) *batchv1.Job {
	name := fmt.Sprintf("%s-maintenance-%s-%s", cluster.Name, action, SanitizeVersion(cluster.Spec.Version))
	component := fmt.Sprintf("maintenance-%s", action)
	labels := CommonLabels(cluster.Name, component)

	image := fmt.Sprintf("ghcr.io/peerdb-io/flow-maintenance:stable-%s", cluster.Spec.Version)
	backoffLimit := int32Ptr(4)

	if spec := cluster.Spec.Maintenance; spec != nil {
		if spec.Image != nil {
			image = *spec.Image
		}
		if spec.BackoffLimit != nil {
			backoffLimit = spec.BackoffLimit
		}
	}

	catalogSecret := cluster.Spec.Dependencies.Catalog.PasswordSecretRef

	container := corev1.Container{
		Name:    fmt.Sprintf("maintenance-%s", action),
		Image:   image,
		Command: []string{"/root/peer-flow", "maintenance", action},
		EnvFrom: []corev1.EnvFromSource{
			{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-config", cluster.Name),
					},
				},
			},
		},
		Env: []corev1.EnvVar{
			{
				Name: "PEERDB_CATALOG_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: catalogSecret.Name,
						},
						Key: catalogSecret.Key,
					},
				},
			},
		},
	}

	if spec := cluster.Spec.Maintenance; spec != nil && spec.Resources != nil {
		container.Resources = *spec.Resources
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: cluster.Name,
					Containers:         []corev1.Container{container},
				},
			},
		},
	}
}
