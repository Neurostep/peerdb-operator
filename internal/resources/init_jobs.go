package resources

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

const defaultTemporalAdminToolsImage = "temporalio/admin-tools:1.24.2-tctl-1.18.1-cli-0.13.2"

func BuildNamespaceRegistrationJob(cluster *v1alpha1.PeerDBCluster) *batchv1.Job {
	name := fmt.Sprintf("%s-temporal-ns-register-%s", cluster.Name, SanitizeVersion(cluster.Spec.Version))
	component := "init-ns-register"
	labels := CommonLabels(cluster.Name, component)
	temporal := cluster.Spec.Dependencies.Temporal

	image := defaultTemporalAdminToolsImage
	backoffLimit := int32Ptr(100)

	if spec := cluster.Spec.Init; spec != nil && spec.TemporalNamespaceRegistration != nil {
		job := spec.TemporalNamespaceRegistration
		if job.Image != nil {
			image = *job.Image
		}
		if job.BackoffLimit != nil {
			backoffLimit = job.BackoffLimit
		}
	}

	command := fmt.Sprintf(
		`until tctl --address %s cluster health | grep -q SERVING; do echo "Waiting for Temporal..."; sleep 2; done; tctl --address %s --ns %s namespace register || tctl --address %s --ns %s namespace describe`,
		temporal.Address, temporal.Address, temporal.Namespace, temporal.Address, temporal.Namespace,
	)

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
					Containers: []corev1.Container{
						{
							Name:    "ns-register",
							Image:   image,
							Command: []string{"sh", "-c", command},
							Env: []corev1.EnvVar{
								{
									Name:  "TEMPORAL_ADDRESS",
									Value: temporal.Address,
								},
								{
									Name:  "TEMPORAL_CLI_ADDRESS",
									Value: temporal.Address,
								},
							},
						},
					},
				},
			},
		},
	}
}

func BuildSearchAttributeJob(cluster *v1alpha1.PeerDBCluster) *batchv1.Job {
	name := fmt.Sprintf("%s-temporal-search-attr-%s", cluster.Name, SanitizeVersion(cluster.Spec.Version))
	component := "init-search-attr"
	labels := CommonLabels(cluster.Name, component)
	temporal := cluster.Spec.Dependencies.Temporal

	image := defaultTemporalAdminToolsImage
	backoffLimit := int32Ptr(100)

	if spec := cluster.Spec.Init; spec != nil && spec.TemporalSearchAttributes != nil {
		job := spec.TemporalSearchAttributes
		if job.Image != nil {
			image = *job.Image
		}
		if job.BackoffLimit != nil {
			backoffLimit = job.BackoffLimit
		}
	}

	command := fmt.Sprintf(
		`until tctl --address %s cluster health | grep -q SERVING; do echo "Waiting for Temporal..."; sleep 2; done; tctl --auto_confirm --address %s --ns %s admin cluster add-search-attributes --name MirrorName --type Keyword`,
		temporal.Address, temporal.Address, temporal.Namespace,
	)

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
					Containers: []corev1.Container{
						{
							Name:    "search-attr",
							Image:   image,
							Command: []string{"sh", "-c", command},
							Env: []corev1.EnvVar{
								{
									Name:  "TEMPORAL_ADDRESS",
									Value: temporal.Address,
								},
								{
									Name:  "TEMPORAL_CLI_ADDRESS",
									Value: temporal.Address,
								},
							},
						},
					},
				},
			},
		},
	}
}
