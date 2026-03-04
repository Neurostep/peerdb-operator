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

func BuildPeerDBServerDeployment(cluster *v1alpha1.PeerDBCluster, configHash string) *appsv1.Deployment {
	name := fmt.Sprintf("%s-server", cluster.Name)
	component := "server"
	labels := CommonLabels(cluster.Name, component)
	selectorLabels := SelectorLabels(cluster.Name, component)

	image := fmt.Sprintf("ghcr.io/peerdb-io/peerdb-server:stable-%s", cluster.Spec.Version)
	replicas := int32Ptr(1)
	var resources corev1.ResourceRequirements

	if spec := cluster.Spec.Components; spec != nil && spec.PeerDBServer != nil {
		if spec.PeerDBServer.Image != nil {
			image = *spec.PeerDBServer.Image
		}
		if spec.PeerDBServer.Replicas != nil {
			replicas = spec.PeerDBServer.Replicas
		}
		if spec.PeerDBServer.Resources != nil {
			resources = *spec.PeerDBServer.Resources
		}
	}

	if resources.Requests == nil && resources.Limits == nil {
		resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		}
	}

	catalogSecret := cluster.Spec.Dependencies.Catalog.PasswordSecretRef
	env := []corev1.EnvVar{
		{
			Name:  "PEERDB_FLOW_SERVER_ADDRESS",
			Value: fmt.Sprintf("grpc://%s-flow-api.%s.svc.cluster.local:8112", cluster.Name, cluster.Namespace),
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

	if spec := cluster.Spec.Components; spec != nil && spec.PeerDBServer != nil && spec.PeerDBServer.PasswordSecretRef != nil {
		ref := spec.PeerDBServer.PasswordSecretRef
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
							Name:  "peerdb-server",
							Image: image,
							Ports: []corev1.ContainerPort{
								{Name: "pg", ContainerPort: 9900, Protocol: corev1.ProtocolTCP},
							},
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
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(9900),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(9900),
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

func BuildPeerDBServerService(cluster *v1alpha1.PeerDBCluster) *corev1.Service {
	name := fmt.Sprintf("%s-server", cluster.Name)
	component := "server"
	labels := CommonLabels(cluster.Name, component)
	selectorLabels := SelectorLabels(cluster.Name, component)

	svcType := corev1.ServiceTypeClusterIP
	var annotations map[string]string
	if spec := cluster.Spec.Components; spec != nil && spec.PeerDBServer != nil && spec.PeerDBServer.Service != nil {
		if spec.PeerDBServer.Service.Type != "" {
			svcType = corev1.ServiceType(spec.PeerDBServer.Service.Type)
		}
		annotations = spec.PeerDBServer.Service.Annotations
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
				{Name: "pg", Port: 9900, TargetPort: intstr.FromString("pg"), Protocol: corev1.ProtocolTCP},
			},
		},
	}
}
