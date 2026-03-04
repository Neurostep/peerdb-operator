package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

func BuildConfigMap(cluster *v1alpha1.PeerDBCluster) *corev1.ConfigMap {
	catalog := cluster.Spec.Dependencies.Catalog
	temporal := cluster.Spec.Dependencies.Temporal

	port := catalog.Port
	if port == 0 {
		port = 5432
	}
	sslMode := catalog.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    CommonLabels(cluster.Name, "config"),
		},
		Data: map[string]string{
			"PEERDB_CATALOG_HOST":     catalog.Host,
			"PEERDB_CATALOG_PORT":     fmt.Sprintf("%d", port),
			"PEERDB_CATALOG_DATABASE": catalog.Database,
			"PEERDB_CATALOG_USER":     catalog.User,
			"PEERDB_CATALOG_SSL_MODE": sslMode,
			"TEMPORAL_HOST_PORT":      temporal.Address,
			"TEMPORAL_NAMESPACE":      temporal.Namespace,
		},
	}
}
