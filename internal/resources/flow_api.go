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

func BuildFlowAPIDeployment(cluster *v1alpha1.PeerDBCluster, configHash string) *appsv1.Deployment {
	name := fmt.Sprintf("%s-flow-api", cluster.Name)
	component := "flow-api"
	labels := CommonLabels(cluster.Name, component)
	selectorLabels := SelectorLabels(cluster.Name, component)

	image := fmt.Sprintf("ghcr.io/peerdb-io/flow-api:stable-%s", cluster.Spec.Version)
	replicas := int32Ptr(1)
	var resources corev1.ResourceRequirements

	if spec := cluster.Spec.Components; spec != nil && spec.FlowAPI != nil {
		if spec.FlowAPI.Image != nil {
			image = *spec.FlowAPI.Image
		}
		if spec.FlowAPI.Replicas != nil {
			replicas = spec.FlowAPI.Replicas
		}
		if spec.FlowAPI.Resources != nil {
			resources = *spec.FlowAPI.Resources
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
							Name:  "flow-api",
							Image: image,
							Ports: []corev1.ContainerPort{
								{Name: "grpc", ContainerPort: 8112, Protocol: corev1.ProtocolTCP},
								{Name: "http", ContainerPort: 8113, Protocol: corev1.ProtocolTCP},
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
							Resources: resources,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									GRPC: &corev1.GRPCAction{
										Port: 8112,
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									GRPC: &corev1.GRPCAction{
										Port: 8112,
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

func BuildFlowAPIService(cluster *v1alpha1.PeerDBCluster) *corev1.Service {
	name := fmt.Sprintf("%s-flow-api", cluster.Name)
	component := "flow-api"
	labels := CommonLabels(cluster.Name, component)
	selectorLabels := SelectorLabels(cluster.Name, component)

	svcType := corev1.ServiceTypeClusterIP
	var annotations map[string]string
	if spec := cluster.Spec.Components; spec != nil && spec.FlowAPI != nil && spec.FlowAPI.Service != nil {
		if spec.FlowAPI.Service.Type != "" {
			svcType = corev1.ServiceType(spec.FlowAPI.Service.Type)
		}
		annotations = spec.FlowAPI.Service.Annotations
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
				{Name: "grpc", Port: 8112, TargetPort: intstr.FromString("grpc"), Protocol: corev1.ProtocolTCP},
				{Name: "http", Port: 8113, TargetPort: intstr.FromString("http"), Protocol: corev1.ProtocolTCP},
			},
		},
	}
}
