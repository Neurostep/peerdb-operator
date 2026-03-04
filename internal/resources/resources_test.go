package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

func testCluster() *v1alpha1.PeerDBCluster {
	return &v1alpha1.PeerDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-peerdb",
			Namespace: "default",
		},
		Spec: v1alpha1.PeerDBClusterSpec{
			Version: "v0.36.7",
			Dependencies: v1alpha1.DependenciesSpec{
				Catalog: v1alpha1.CatalogSpec{
					Host:     "catalog.example.com",
					Port:     5432,
					Database: "peerdb",
					User:     "peerdb",
					PasswordSecretRef: v1alpha1.SecretKeySelector{
						Name: "catalog-password",
						Key:  "password",
					},
				},
				Temporal: v1alpha1.TemporalSpec{
					Address:   "temporal.example.com:7233",
					Namespace: "peerdb",
				},
			},
		},
	}
}

func testWorkerPool() *v1alpha1.PeerDBWorkerPool {
	return &v1alpha1.PeerDBWorkerPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workers",
			Namespace: "default",
		},
		Spec: v1alpha1.PeerDBWorkerPoolSpec{
			ClusterRef: "test-peerdb",
		},
	}
}

func testSnapshotPool() *v1alpha1.PeerDBSnapshotPool {
	return &v1alpha1.PeerDBSnapshotPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-snapshots",
			Namespace: "default",
		},
		Spec: v1alpha1.PeerDBSnapshotPoolSpec{
			ClusterRef: "test-peerdb",
			Storage: v1alpha1.SnapshotPoolStorageSpec{
				Size: resource.MustParse("10Gi"),
			},
		},
	}
}

func stringPtr(s string) *string { return &s }

func TestCommonLabels(t *testing.T) {
	labels := CommonLabels("my-cluster", "flow-api")
	expected := map[string]string{
		"app.kubernetes.io/name":       "peerdb",
		"app.kubernetes.io/instance":   "my-cluster",
		"app.kubernetes.io/component":  "flow-api",
		"app.kubernetes.io/managed-by": "peerdb-operator",
	}
	for k, v := range expected {
		if labels[k] != v {
			t.Errorf("CommonLabels[%s] = %q, want %q", k, labels[k], v)
		}
	}
}

func TestSelectorLabels(t *testing.T) {
	labels := SelectorLabels("my-cluster", "server")
	if labels["app.kubernetes.io/name"] != "peerdb" {
		t.Errorf("unexpected name label: %s", labels["app.kubernetes.io/name"])
	}
	if labels["app.kubernetes.io/instance"] != "my-cluster" {
		t.Errorf("unexpected instance label: %s", labels["app.kubernetes.io/instance"])
	}
	if _, ok := labels["app.kubernetes.io/managed-by"]; ok {
		t.Error("SelectorLabels should not include managed-by")
	}
}

func TestBuildConfigMap(t *testing.T) {
	cluster := testCluster()

	t.Run("defaults", func(t *testing.T) {
		cm := BuildConfigMap(cluster)
		if cm.Name != "test-peerdb-config" {
			t.Errorf("name = %q, want %q", cm.Name, "test-peerdb-config")
		}
		if cm.Namespace != "default" {
			t.Errorf("namespace = %q, want %q", cm.Namespace, "default")
		}
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_HOST", "catalog.example.com")
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_PORT", "5432")
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_DATABASE", "peerdb")
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_USER", "peerdb")
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_SSL_MODE", "require")
		assertMapValue(t, cm.Data, "TEMPORAL_HOST_PORT", "temporal.example.com:7233")
		assertMapValue(t, cm.Data, "TEMPORAL_NAMESPACE", "peerdb")
	})

	t.Run("custom port and ssl", func(t *testing.T) {
		c := testCluster()
		c.Spec.Dependencies.Catalog.Port = 5433
		c.Spec.Dependencies.Catalog.SSLMode = "disable"
		cm := BuildConfigMap(c)
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_PORT", "5433")
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_SSL_MODE", "disable")
	})

	t.Run("default port when zero", func(t *testing.T) {
		c := testCluster()
		c.Spec.Dependencies.Catalog.Port = 0
		cm := BuildConfigMap(c)
		assertMapValue(t, cm.Data, "PEERDB_CATALOG_PORT", "5432")
	})
}

func TestBuildServiceAccount(t *testing.T) {
	t.Run("default creates SA", func(t *testing.T) {
		cluster := testCluster()
		sa := BuildServiceAccount(cluster)
		if sa == nil {
			t.Fatal("expected ServiceAccount, got nil")
		}
		if sa.Name != "test-peerdb" {
			t.Errorf("name = %q, want %q", sa.Name, "test-peerdb")
		}
		if sa.Namespace != "default" {
			t.Errorf("namespace = %q, want %q", sa.Namespace, "default")
		}
	})

	t.Run("create false returns nil", func(t *testing.T) {
		cluster := testCluster()
		cluster.Spec.ServiceAccount = &v1alpha1.ServiceAccountConfig{Create: false}
		sa := BuildServiceAccount(cluster)
		if sa != nil {
			t.Error("expected nil when Create=false")
		}
	})

	t.Run("custom annotations", func(t *testing.T) {
		cluster := testCluster()
		cluster.Spec.ServiceAccount = &v1alpha1.ServiceAccountConfig{
			Create:      true,
			Annotations: map[string]string{"iam.gke.io/gcp-service-account": "test@proj.iam"},
		}
		sa := BuildServiceAccount(cluster)
		if sa == nil {
			t.Fatal("expected ServiceAccount")
		}
		if sa.Annotations["iam.gke.io/gcp-service-account"] != "test@proj.iam" {
			t.Error("annotation not set")
		}
	})
}

func TestBuildFlowAPIDeployment(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cluster := testCluster()
		dep := BuildFlowAPIDeployment(cluster, "testhash")
		if dep.Name != "test-peerdb-flow-api" {
			t.Errorf("name = %q", dep.Name)
		}
		if dep.Namespace != "default" {
			t.Errorf("namespace = %q", dep.Namespace)
		}
		if *dep.Spec.Replicas != 1 {
			t.Errorf("replicas = %d, want 1", *dep.Spec.Replicas)
		}
		container := dep.Spec.Template.Spec.Containers[0]
		if container.Image != "ghcr.io/peerdb-io/flow-api:stable-v0.36.7" {
			t.Errorf("image = %q", container.Image)
		}
		assertContainerPort(t, container, "grpc", 8112)
		assertContainerPort(t, container, "http", 8113)
		if container.EnvFrom[0].ConfigMapRef.Name != "test-peerdb-config" {
			t.Errorf("configmap ref = %q", container.EnvFrom[0].ConfigMapRef.Name)
		}
		assertSecretEnvVar(t, container, "PEERDB_CATALOG_PASSWORD", "catalog-password", "password")
		if container.Resources.Requests.Cpu().String() != "100m" {
			t.Errorf("cpu request = %s", container.Resources.Requests.Cpu())
		}
		if container.LivenessProbe == nil || container.LivenessProbe.GRPC == nil {
			t.Error("expected GRPC liveness probe")
		}
		if container.ReadinessProbe == nil || container.ReadinessProbe.GRPC == nil {
			t.Error("expected GRPC readiness probe")
		}
		if dep.Spec.Template.Spec.ServiceAccountName != "test-peerdb" {
			t.Errorf("serviceAccountName = %q", dep.Spec.Template.Spec.ServiceAccountName)
		}
	})

	t.Run("custom overrides", func(t *testing.T) {
		cluster := testCluster()
		replicas := int32(3)
		cluster.Spec.Components = &v1alpha1.ComponentsSpec{
			FlowAPI: &v1alpha1.FlowAPISpec{
				Image:    stringPtr("custom-image:latest"),
				Replicas: &replicas,
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("2"),
					},
				},
			},
		}
		dep := BuildFlowAPIDeployment(cluster, "testhash")
		if *dep.Spec.Replicas != 3 {
			t.Errorf("replicas = %d, want 3", *dep.Spec.Replicas)
		}
		if dep.Spec.Template.Spec.Containers[0].Image != "custom-image:latest" {
			t.Errorf("image = %q", dep.Spec.Template.Spec.Containers[0].Image)
		}
		if dep.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String() != "2" {
			t.Errorf("cpu = %s", dep.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu())
		}
	})
}

func TestBuildFlowAPIService(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cluster := testCluster()
		svc := BuildFlowAPIService(cluster)
		if svc.Name != "test-peerdb-flow-api" {
			t.Errorf("name = %q", svc.Name)
		}
		if svc.Spec.Type != corev1.ServiceTypeClusterIP {
			t.Errorf("type = %q, want ClusterIP", svc.Spec.Type)
		}
		assertServicePort(t, svc, "grpc", 8112)
		assertServicePort(t, svc, "http", 8113)
	})

	t.Run("custom type", func(t *testing.T) {
		cluster := testCluster()
		cluster.Spec.Components = &v1alpha1.ComponentsSpec{
			FlowAPI: &v1alpha1.FlowAPISpec{
				Service: &v1alpha1.ServiceSpec{Type: "LoadBalancer"},
			},
		}
		svc := BuildFlowAPIService(cluster)
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			t.Errorf("type = %q, want LoadBalancer", svc.Spec.Type)
		}
	})
}

func TestBuildPeerDBServerDeployment(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cluster := testCluster()
		dep := BuildPeerDBServerDeployment(cluster, "testhash")
		if dep.Name != "test-peerdb-server" {
			t.Errorf("name = %q", dep.Name)
		}
		container := dep.Spec.Template.Spec.Containers[0]
		assertContainerPort(t, container, "pg", 9900)
		assertEnvVarValue(t, container, "PEERDB_FLOW_SERVER_ADDRESS", "grpc://test-peerdb-flow-api.default.svc.cluster.local:8112")
		assertSecretEnvVar(t, container, "PEERDB_CATALOG_PASSWORD", "catalog-password", "password")
	})

	t.Run("with password secret", func(t *testing.T) {
		cluster := testCluster()
		cluster.Spec.Components = &v1alpha1.ComponentsSpec{
			PeerDBServer: &v1alpha1.PeerDBServerSpec{
				PasswordSecretRef: &v1alpha1.SecretKeySelector{Name: "server-pwd", Key: "pwd"},
			},
		}
		dep := BuildPeerDBServerDeployment(cluster, "testhash")
		container := dep.Spec.Template.Spec.Containers[0]
		assertSecretEnvVar(t, container, "PEERDB_PASSWORD", "server-pwd", "pwd")
	})
}

func TestBuildPeerDBServerService(t *testing.T) {
	cluster := testCluster()
	svc := BuildPeerDBServerService(cluster)
	if svc.Name != "test-peerdb-server" {
		t.Errorf("name = %q", svc.Name)
	}
	assertServicePort(t, svc, "pg", 9900)
}

func TestBuildUIDeployment(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cluster := testCluster()
		dep := BuildUIDeployment(cluster, "testhash")
		if dep.Name != "test-peerdb-ui" {
			t.Errorf("name = %q", dep.Name)
		}
		container := dep.Spec.Template.Spec.Containers[0]
		assertContainerPort(t, container, "http", 3000)
		assertEnvVarValue(t, container, "NEXTAUTH_URL", "http://localhost:3000")
		assertEnvVarValue(t, container, "PEERDB_FLOW_SERVER_HTTP", "http://test-peerdb-flow-api.default.svc.cluster.local:8113")
	})

	t.Run("custom NextAuthURL", func(t *testing.T) {
		cluster := testCluster()
		cluster.Spec.Components = &v1alpha1.ComponentsSpec{
			UI: &v1alpha1.UISpec{
				NextAuthURL: stringPtr("https://peerdb.example.com"),
			},
		}
		dep := BuildUIDeployment(cluster, "testhash")
		container := dep.Spec.Template.Spec.Containers[0]
		assertEnvVarValue(t, container, "NEXTAUTH_URL", "https://peerdb.example.com")
	})

	t.Run("with password and nextauth secrets", func(t *testing.T) {
		cluster := testCluster()
		cluster.Spec.Components = &v1alpha1.ComponentsSpec{
			UI: &v1alpha1.UISpec{
				PasswordSecretRef: &v1alpha1.SecretKeySelector{Name: "ui-pwd", Key: "pwd"},
				NextAuthSecretRef: &v1alpha1.SecretKeySelector{Name: "nextauth", Key: "secret"},
			},
		}
		dep := BuildUIDeployment(cluster, "testhash")
		container := dep.Spec.Template.Spec.Containers[0]
		assertSecretEnvVar(t, container, "PEERDB_PASSWORD", "ui-pwd", "pwd")
		assertSecretEnvVar(t, container, "NEXTAUTH_SECRET", "nextauth", "secret")
	})
}

func TestBuildUIService(t *testing.T) {
	cluster := testCluster()
	svc := BuildUIService(cluster)
	if svc.Name != "test-peerdb-ui" {
		t.Errorf("name = %q", svc.Name)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("type = %q, want LoadBalancer", svc.Spec.Type)
	}
	assertServicePort(t, svc, "http", 3000)
}

func TestBuildNamespaceRegistrationJob(t *testing.T) {
	cluster := testCluster()
	job := BuildNamespaceRegistrationJob(cluster)
	if job.Name != "test-peerdb-temporal-ns-register-v0-36-7" {
		t.Errorf("name = %q", job.Name)
	}
	if job.Namespace != "default" {
		t.Errorf("namespace = %q", job.Namespace)
	}
	if *job.Spec.BackoffLimit != 100 {
		t.Errorf("backoffLimit = %d, want 100", *job.Spec.BackoffLimit)
	}
	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != defaultTemporalAdminToolsImage {
		t.Errorf("image = %q", container.Image)
	}
	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("restartPolicy = %q", job.Spec.Template.Spec.RestartPolicy)
	}
	assertEnvVarValue(t, container, "TEMPORAL_ADDRESS", "temporal.example.com:7233")
	assertEnvVarValue(t, container, "TEMPORAL_CLI_ADDRESS", "temporal.example.com:7233")
}

func TestBuildSearchAttributeJob(t *testing.T) {
	cluster := testCluster()
	job := BuildSearchAttributeJob(cluster)
	if job.Name != "test-peerdb-temporal-search-attr-v0-36-7" {
		t.Errorf("name = %q", job.Name)
	}
	container := job.Spec.Template.Spec.Containers[0]
	cmd := container.Command[2]
	if len(cmd) == 0 {
		t.Fatal("empty command")
	}
	found := false
	for _, s := range container.Command {
		if contains(s, "add-search-attributes") {
			found = true
			break
		}
	}
	if !found {
		t.Error("command should contain add-search-attributes")
	}
}

func TestBuildFlowWorkerDeployment(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		pool := testWorkerPool()
		cluster := testCluster()
		dep := BuildFlowWorkerDeployment(pool, cluster, "testhash")
		if dep.Name != "test-workers" {
			t.Errorf("name = %q", dep.Name)
		}
		if *dep.Spec.Replicas != 2 {
			t.Errorf("replicas = %d, want 2", *dep.Spec.Replicas)
		}
		container := dep.Spec.Template.Spec.Containers[0]
		if container.Resources.Requests.Cpu().String() != "2" {
			t.Errorf("cpu request = %s, want 2", container.Resources.Requests.Cpu())
		}
		if container.Resources.Requests.Memory().String() != "8Gi" {
			t.Errorf("memory request = %s, want 8Gi", container.Resources.Requests.Memory())
		}
		if dep.Labels["peerdb.io/pool"] != "test-workers" {
			t.Errorf("pool label = %q", dep.Labels["peerdb.io/pool"])
		}
		if container.EnvFrom[0].ConfigMapRef.Name != "test-peerdb-config" {
			t.Errorf("configmap ref = %q", container.EnvFrom[0].ConfigMapRef.Name)
		}
		if dep.Spec.Template.Spec.Affinity == nil || dep.Spec.Template.Spec.Affinity.PodAntiAffinity == nil {
			t.Error("expected default pod anti-affinity")
		}
	})

	t.Run("custom affinity replaces default", func(t *testing.T) {
		pool := testWorkerPool()
		pool.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{},
		}
		cluster := testCluster()
		dep := BuildFlowWorkerDeployment(pool, cluster, "testhash")
		if dep.Spec.Template.Spec.Affinity.PodAntiAffinity != nil {
			t.Error("custom affinity should replace default anti-affinity")
		}
		if dep.Spec.Template.Spec.Affinity.NodeAffinity == nil {
			t.Error("expected custom node affinity")
		}
	})

	t.Run("extra env and labels", func(t *testing.T) {
		pool := testWorkerPool()
		pool.Spec.ExtraEnv = []corev1.EnvVar{{Name: "MY_VAR", Value: "my-val"}}
		pool.Spec.PodLabels = map[string]string{"team": "data"}
		cluster := testCluster()
		dep := BuildFlowWorkerDeployment(pool, cluster, "testhash")
		container := dep.Spec.Template.Spec.Containers[0]
		assertEnvVarValue(t, container, "MY_VAR", "my-val")
		if dep.Labels["team"] != "data" {
			t.Error("expected custom pod label")
		}
	})
}

func TestBuildSnapshotWorkerStatefulSet(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		pool := testSnapshotPool()
		cluster := testCluster()
		sts := BuildSnapshotWorkerStatefulSet(pool, cluster, "testhash")
		if sts.Name != "test-snapshots" {
			t.Errorf("name = %q", sts.Name)
		}
		if sts.Spec.ServiceName != "test-snapshots" {
			t.Errorf("serviceName = %q", sts.Spec.ServiceName)
		}
		if *sts.Spec.Replicas != 1 {
			t.Errorf("replicas = %d, want 1", *sts.Spec.Replicas)
		}
		if *sts.Spec.Template.Spec.TerminationGracePeriodSeconds != 600 {
			t.Errorf("terminationGracePeriod = %d, want 600", *sts.Spec.Template.Spec.TerminationGracePeriodSeconds)
		}
		if len(sts.Spec.VolumeClaimTemplates) != 1 {
			t.Fatalf("expected 1 VCT, got %d", len(sts.Spec.VolumeClaimTemplates))
		}
		vct := sts.Spec.VolumeClaimTemplates[0]
		if vct.Name != "data" {
			t.Errorf("VCT name = %q", vct.Name)
		}
		storageReq := vct.Spec.Resources.Requests[corev1.ResourceStorage]
		if storageReq.String() != "10Gi" {
			t.Errorf("storage = %s, want 10Gi", storageReq.String())
		}
		container := sts.Spec.Template.Spec.Containers[0]
		if container.VolumeMounts[0].MountPath != "/data" {
			t.Errorf("mountPath = %q", container.VolumeMounts[0].MountPath)
		}
		if sts.Labels["peerdb.io/pool"] != "test-snapshots" {
			t.Errorf("pool label = %q", sts.Labels["peerdb.io/pool"])
		}
	})

	t.Run("custom storage class", func(t *testing.T) {
		pool := testSnapshotPool()
		pool.Spec.Storage.StorageClassName = stringPtr("fast-ssd")
		cluster := testCluster()
		sts := BuildSnapshotWorkerStatefulSet(pool, cluster, "testhash")
		vct := sts.Spec.VolumeClaimTemplates[0]
		if vct.Spec.StorageClassName == nil || *vct.Spec.StorageClassName != "fast-ssd" {
			t.Error("expected storageClassName=fast-ssd")
		}
	})
}

func TestBuildSnapshotWorkerService(t *testing.T) {
	pool := testSnapshotPool()
	svc := BuildSnapshotWorkerService(pool)
	if svc.Name != "test-snapshots" {
		t.Errorf("name = %q", svc.Name)
	}
	if svc.Spec.ClusterIP != corev1.ClusterIPNone {
		t.Errorf("ClusterIP = %q, want None (headless)", svc.Spec.ClusterIP)
	}
	if svc.Spec.Selector["peerdb.io/pool"] != "test-snapshots" {
		t.Errorf("selector pool label = %q", svc.Spec.Selector["peerdb.io/pool"])
	}
}

// --- test helpers ---

func assertMapValue(t *testing.T, m map[string]string, key, want string) {
	t.Helper()
	if got := m[key]; got != want {
		t.Errorf("data[%q] = %q, want %q", key, got, want)
	}
}

func assertContainerPort(t *testing.T, c corev1.Container, name string, port int32) {
	t.Helper()
	for _, p := range c.Ports {
		if p.Name == name {
			if p.ContainerPort != port {
				t.Errorf("port %q = %d, want %d", name, p.ContainerPort, port)
			}
			return
		}
	}
	t.Errorf("port %q not found", name)
}

func assertServicePort(t *testing.T, svc *corev1.Service, name string, port int32) {
	t.Helper()
	for _, p := range svc.Spec.Ports {
		if p.Name == name {
			if p.Port != port {
				t.Errorf("service port %q = %d, want %d", name, p.Port, port)
			}
			return
		}
	}
	t.Errorf("service port %q not found", name)
}

func assertEnvVarValue(t *testing.T, c corev1.Container, name, want string) {
	t.Helper()
	for _, e := range c.Env {
		if e.Name == name {
			if e.Value != want {
				t.Errorf("env %q = %q, want %q", name, e.Value, want)
			}
			return
		}
	}
	t.Errorf("env %q not found", name)
}

func assertSecretEnvVar(t *testing.T, c corev1.Container, envName, secretName, secretKey string) {
	t.Helper()
	for _, e := range c.Env {
		if e.Name == envName {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				t.Errorf("env %q has no secretKeyRef", envName)
				return
			}
			if e.ValueFrom.SecretKeyRef.Name != secretName {
				t.Errorf("env %q secret name = %q, want %q", envName, e.ValueFrom.SecretKeyRef.Name, secretName)
			}
			if e.ValueFrom.SecretKeyRef.Key != secretKey {
				t.Errorf("env %q secret key = %q, want %q", envName, e.ValueFrom.SecretKeyRef.Key, secretKey)
			}
			return
		}
	}
	t.Errorf("env %q not found", envName)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestComputeConfigHash(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		data := map[string]string{"A": "1", "B": "2"}
		secrets := map[string]string{"s1": "rv1"}
		h1 := ComputeConfigHash(data, secrets)
		h2 := ComputeConfigHash(data, secrets)
		if h1 != h2 {
			t.Errorf("hash not deterministic: %q != %q", h1, h2)
		}
	})

	t.Run("changes on data change", func(t *testing.T) {
		h1 := ComputeConfigHash(map[string]string{"A": "1"}, nil)
		h2 := ComputeConfigHash(map[string]string{"A": "2"}, nil)
		if h1 == h2 {
			t.Error("hash should change when config data changes")
		}
	})

	t.Run("changes on secret rv change", func(t *testing.T) {
		data := map[string]string{"A": "1"}
		h1 := ComputeConfigHash(data, map[string]string{"s1": "rv1"})
		h2 := ComputeConfigHash(data, map[string]string{"s1": "rv2"})
		if h1 == h2 {
			t.Error("hash should change when secret resource version changes")
		}
	})

	t.Run("nil maps", func(t *testing.T) {
		h := ComputeConfigHash(nil, nil)
		if h == "" {
			t.Error("hash should not be empty for nil maps")
		}
	})
}

func TestSanitizeVersion(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"v0.36.7", "v0-36-7"},
		{"v1.2.3", "v1-2-3"},
		{"latest", "latest"},
	}
	for _, tt := range tests {
		got := SanitizeVersion(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
