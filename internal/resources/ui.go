package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

func BuildUIDeployment(cluster *v1alpha1.PeerDBCluster, configHash string) *appsv1.Deployment {
	name := fmt.Sprintf("%s-ui", cluster.Name)
	component := "ui"
	labels := CommonLabels(cluster.Name, component)
	selectorLabels := SelectorLabels(cluster.Name, component)

	image := fmt.Sprintf("ghcr.io/peerdb-io/peerdb-ui:stable-%s", cluster.Spec.Version)
	replicas := int32Ptr(1)
	var resources corev1.ResourceRequirements

	if spec := cluster.Spec.Components; spec != nil && spec.UI != nil {
		if spec.UI.Image != nil {
			image = *spec.UI.Image
		}
		if spec.UI.Replicas != nil {
			replicas = spec.UI.Replicas
		}
		if spec.UI.Resources != nil {
			resources = *spec.UI.Resources
		}
	}

	if resources.Requests == nil && resources.Limits == nil {
		resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		}
	}

	catalogSecret := cluster.Spec.Dependencies.Catalog.PasswordSecretRef
	env := []corev1.EnvVar{
		{
			Name:  "PEERDB_FLOW_SERVER_HTTP",
			Value: fmt.Sprintf("http://%s-flow-api.%s.svc.cluster.local:8113", cluster.Name, cluster.Namespace),
		},
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
	}

	nextAuthURL := "http://localhost:3000"
	if spec := cluster.Spec.Components; spec != nil && spec.UI != nil && spec.UI.NextAuthURL != nil {
		nextAuthURL = *spec.UI.NextAuthURL
	}
	env = append(env, corev1.EnvVar{
		Name:  "NEXTAUTH_URL",
		Value: nextAuthURL,
	})

	if spec := cluster.Spec.Components; spec != nil && spec.UI != nil {
		if spec.UI.PasswordSecretRef != nil {
			ref := spec.UI.PasswordSecretRef
			env = append(env, corev1.EnvVar{
				Name: "PEERDB_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: ref.Name,
						},
						Key: ref.Key,
					},
				},
			})
		}
		if spec.UI.NextAuthSecretRef != nil {
			ref := spec.UI.NextAuthSecretRef
			env = append(env, corev1.EnvVar{
				Name: "NEXTAUTH_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: ref.Name,
						},
						Key: ref.Key,
					},
				},
			})
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						AnnotationConfigHash: configHash,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: cluster.Name,
					Containers: []corev1.Container{
						{
							Name:  "peerdb-ui",
							Image: image,
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 3000, Protocol: corev1.ProtocolTCP},
							},
							Env:       env,
							Resources: resources,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(3000),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(3000),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
}

func BuildUIService(cluster *v1alpha1.PeerDBCluster) *corev1.Service {
	name := fmt.Sprintf("%s-ui", cluster.Name)
	component := "ui"
	labels := CommonLabels(cluster.Name, component)
	selectorLabels := SelectorLabels(cluster.Name, component)

	svcType := corev1.ServiceTypeLoadBalancer
	var annotations map[string]string
	if spec := cluster.Spec.Components; spec != nil && spec.UI != nil && spec.UI.Service != nil {
		if spec.UI.Service.Type != "" {
			svcType = corev1.ServiceType(spec.UI.Service.Type)
		}
		annotations = spec.UI.Service.Annotations
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 3000, TargetPort: intstr.FromString("http"), Protocol: corev1.ProtocolTCP},
			},
		},
	}
}
