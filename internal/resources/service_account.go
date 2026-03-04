package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

func BuildServiceAccount(cluster *v1alpha1.PeerDBCluster) *corev1.ServiceAccount {
	sa := cluster.Spec.ServiceAccount

	// Default to creating the ServiceAccount if not specified.
	if sa != nil && !sa.Create {
		return nil
	}

	var annotations map[string]string
	if sa != nil {
		annotations = sa.Annotations
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name,
			Namespace:   cluster.Namespace,
			Labels:      CommonLabels(cluster.Name, "service-account"),
			Annotations: annotations,
		},
	}
}
