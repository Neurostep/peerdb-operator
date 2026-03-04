package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

func BuildFlowWorkerDeployment(pool *v1alpha1.PeerDBWorkerPool, cluster *v1alpha1.PeerDBCluster, configHash string) *appsv1.Deployment {
	name := pool.Name
	component := "flow-worker"
	labels := CommonLabels(cluster.Name, component)
	selectorLabels := SelectorLabels(cluster.Name, component)

	// Merge pool-specific labels.
	for k, v := range pool.Spec.PodLabels {
		labels[k] = v
	}
	// Add pool name to distinguish multiple pools.
	labels["peerdb.io/pool"] = pool.Name

	image := fmt.Sprintf("ghcr.io/peerdb-io/flow-worker:stable-%s", cluster.Spec.Version)
	if pool.Spec.Image != "" {
		image = pool.Spec.Image
	}

	replicas := int32Ptr(2)
	if pool.Spec.Replicas != nil {
		replicas = pool.Spec.Replicas
	}

	resources := pool.Spec.Resources
	if resources.Requests == nil && resources.Limits == nil {
		resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		}
	}

	catalogSecret := cluster.Spec.Dependencies.Catalog.PasswordSecretRef

	env := make([]corev1.EnvVar, 0, 1+len(pool.Spec.ExtraEnv))
	env = append(env, corev1.EnvVar{
		Name: "PEERDB_CATALOG_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: catalogSecret.Name,
				},
				Key: catalogSecret.Key,
			},
		},
	})
	env = append(env, pool.Spec.ExtraEnv...)

	podAnnotations := make(map[string]string)
	for k, v := range pool.Spec.PodAnnotations {
		podAnnotations[k] = v
	}
	podAnnotations[AnnotationConfigHash] = configHash

	// Pod anti-affinity: prefer different zones.
	affinity := pool.Spec.Affinity
	if affinity == nil {
		affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: selectorLabels,
							},
							TopologyKey: "topology.kubernetes.io/zone",
						},
					},
				},
			},
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pool.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":      "peerdb",
					"app.kubernetes.io/instance":  cluster.Name,
					"app.kubernetes.io/component": component,
					"peerdb.io/pool":              pool.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: cluster.Name,
					NodeSelector:       pool.Spec.NodeSelector,
					Tolerations:        pool.Spec.Tolerations,
					Affinity:           affinity,
					Containers: []corev1.Container{
						{
							Name:  "flow-worker",
							Image: image,
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: fmt.Sprintf("%s-config", cluster.Name),
										},
									},
								},
							},
							Env:       env,
							Resources: resources,
						},
					},
				},
			},
		},
	}
}
