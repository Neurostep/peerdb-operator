package resources

func CommonLabels(clusterName, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "peerdb",
		"app.kubernetes.io/instance":   clusterName,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "peerdb-operator",
	}
}

func SelectorLabels(clusterName, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "peerdb",
		"app.kubernetes.io/instance":  clusterName,
		"app.kubernetes.io/component": component,
	}
}
