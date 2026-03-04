package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

func BuildSnapshotWorkerStatefulSet(pool *v1alpha1.PeerDBSnapshotPool, cluster *v1alpha1.PeerDBCluster, configHash string) *appsv1.StatefulSet {
	name := pool.Name
	component := "snapshot-worker"
	labels := CommonLabels(cluster.Name, component)

	for k, v := range pool.Spec.PodLabels {
		labels[k] = v
	}
	labels["peerdb.io/pool"] = pool.Name

	image := fmt.Sprintf("ghcr.io/peerdb-io/flow-snapshot-worker:stable-%s", cluster.Spec.Version)
	if pool.Spec.Image != "" {
		image = pool.Spec.Image
	}

	replicas := int32Ptr(1)
	if pool.Spec.Replicas != nil {
		replicas = pool.Spec.Replicas
	}

	resources := pool.Spec.Resources
	if resources.Requests == nil && resources.Limits == nil {
		resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
	}

	terminationGracePeriod := int64Ptr(600)
	if pool.Spec.TerminationGracePeriodSeconds != nil {
		terminationGracePeriod = pool.Spec.TerminationGracePeriodSeconds
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

	matchLabels := map[string]string{
		"app.kubernetes.io/name":      "peerdb",
		"app.kubernetes.io/instance":  cluster.Name,
		"app.kubernetes.io/component": component,
		"peerdb.io/pool":              pool.Name,
	}

	storageSize := pool.Spec.Storage.Size

	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: storageSize,
			},
		},
	}
	if pool.Spec.Storage.StorageClassName != nil {
		pvcSpec.StorageClassName = pool.Spec.Storage.StorageClassName
	}

	podAnnotations := make(map[string]string)
	for k, v := range pool.Spec.PodAnnotations {
		podAnnotations[k] = v
	}
	podAnnotations[AnnotationConfigHash] = configHash

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pool.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    replicas,
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            cluster.Name,
					TerminationGracePeriodSeconds: terminationGracePeriod,
					NodeSelector:                  pool.Spec.NodeSelector,
					Tolerations:                   pool.Spec.Tolerations,
					Affinity:                      pool.Spec.Affinity,
					Containers: []corev1.Container{
						{
							Name:  "snapshot-worker",
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: pvcSpec,
				},
			},
		},
	}
}

func BuildSnapshotWorkerService(pool *v1alpha1.PeerDBSnapshotPool) *corev1.Service {
	name := pool.Name
	component := "snapshot-worker"
	labels := CommonLabels(pool.Spec.ClusterRef, component)

	matchLabels := map[string]string{
		"app.kubernetes.io/name":      "peerdb",
		"app.kubernetes.io/instance":  pool.Spec.ClusterRef,
		"app.kubernetes.io/component": component,
		"peerdb.io/pool":              pool.Name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pool.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  matchLabels,
		},
	}
}
