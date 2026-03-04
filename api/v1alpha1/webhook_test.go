/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func int32Ptr(i int32) *int32 { return &i }

func validCluster() *PeerDBCluster {
	return &PeerDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: PeerDBClusterSpec{
			Version: "v0.36.7",
			Dependencies: DependenciesSpec{
				Catalog: CatalogSpec{
					Host:     "catalog.example.com",
					Port:     5432,
					Database: "peerdb",
					User:     "peerdb",
					SSLMode:  "require",
					PasswordSecretRef: SecretKeySelector{
						Name: "catalog-password",
						Key:  "password",
					},
				},
				Temporal: TemporalSpec{
					Address:   "temporal.example.com:7233",
					Namespace: "peerdb",
				},
			},
		},
	}
}

// --- PeerDBCluster Defaulter ---

func TestPeerDBClusterDefaulter_DefaultsCatalogPort(t *testing.T) {
	cluster := validCluster()
	cluster.Spec.Dependencies.Catalog.Port = 0
	cluster.Spec.Dependencies.Catalog.SSLMode = ""

	d := &PeerDBClusterCustomDefaulter{}
	if err := d.Default(context.Background(), cluster); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Spec.Dependencies.Catalog.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cluster.Spec.Dependencies.Catalog.Port)
	}
	if cluster.Spec.Dependencies.Catalog.SSLMode != "require" {
		t.Errorf("expected sslMode 'require', got %q", cluster.Spec.Dependencies.Catalog.SSLMode)
	}
}

func TestPeerDBClusterDefaulter_DefaultsServiceAccount(t *testing.T) {
	cluster := validCluster()
	cluster.Spec.ServiceAccount = nil

	d := &PeerDBClusterCustomDefaulter{}
	if err := d.Default(context.Background(), cluster); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Spec.ServiceAccount == nil || !cluster.Spec.ServiceAccount.Create {
		t.Error("expected ServiceAccount to be defaulted with Create=true")
	}
}

func TestPeerDBClusterDefaulter_DefaultsComponentReplicas(t *testing.T) {
	cluster := validCluster()
	cluster.Spec.Components = &ComponentsSpec{
		FlowAPI:      &FlowAPISpec{},
		PeerDBServer: &PeerDBServerSpec{},
		UI:           &UISpec{},
	}

	d := &PeerDBClusterCustomDefaulter{}
	if err := d.Default(context.Background(), cluster); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Spec.Components.FlowAPI.Replicas == nil || *cluster.Spec.Components.FlowAPI.Replicas != 1 {
		t.Error("expected FlowAPI replicas to be defaulted to 1")
	}
	if cluster.Spec.Components.PeerDBServer.Replicas == nil || *cluster.Spec.Components.PeerDBServer.Replicas != 1 {
		t.Error("expected PeerDBServer replicas to be defaulted to 1")
	}
	if cluster.Spec.Components.UI.Replicas == nil || *cluster.Spec.Components.UI.Replicas != 1 {
		t.Error("expected UI replicas to be defaulted to 1")
	}
}

// --- PeerDBCluster Validator ---

func TestPeerDBClusterValidator_ValidCreate(t *testing.T) {
	v := &PeerDBClusterCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), validCluster())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPeerDBClusterValidator_MissingVersion(t *testing.T) {
	cluster := validCluster()
	cluster.Spec.Version = ""

	v := &PeerDBClusterCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), cluster)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestPeerDBClusterValidator_MissingCatalogHost(t *testing.T) {
	cluster := validCluster()
	cluster.Spec.Dependencies.Catalog.Host = ""

	v := &PeerDBClusterCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), cluster)
	if err == nil {
		t.Fatal("expected error for missing catalog host")
	}
}

func TestPeerDBClusterValidator_MissingTemporalAddress(t *testing.T) {
	cluster := validCluster()
	cluster.Spec.Dependencies.Temporal.Address = ""

	v := &PeerDBClusterCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), cluster)
	if err == nil {
		t.Fatal("expected error for missing temporal address")
	}
}

func TestPeerDBClusterValidator_ImmutableCatalogHost(t *testing.T) {
	old := validCluster()
	new := validCluster()
	new.Spec.Dependencies.Catalog.Host = "new-host.example.com"

	v := &PeerDBClusterCustomValidator{}
	_, err := v.ValidateUpdate(context.Background(), old, new)
	if err == nil {
		t.Fatal("expected error for immutable catalog host change")
	}
}

func TestPeerDBClusterValidator_DeleteAlwaysAllowed(t *testing.T) {
	v := &PeerDBClusterCustomValidator{}
	_, err := v.ValidateDelete(context.Background(), validCluster())
	if err != nil {
		t.Fatalf("expected no error on delete, got %v", err)
	}
}

// --- PeerDBWorkerPool Defaulter ---

func TestPeerDBWorkerPoolDefaulter_DefaultsReplicas(t *testing.T) {
	pool := &PeerDBWorkerPool{
		Spec: PeerDBWorkerPoolSpec{ClusterRef: "test"},
	}

	d := &PeerDBWorkerPoolCustomDefaulter{}
	if err := d.Default(context.Background(), pool); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pool.Spec.Replicas == nil || *pool.Spec.Replicas != 2 {
		t.Error("expected replicas to be defaulted to 2")
	}
}

func TestPeerDBWorkerPoolDefaulter_DefaultsAutoscaling(t *testing.T) {
	pool := &PeerDBWorkerPool{
		Spec: PeerDBWorkerPoolSpec{
			ClusterRef: "test",
			Autoscaling: &AutoscalingSpec{
				Enabled:     true,
				MaxReplicas: 10,
			},
		},
	}

	d := &PeerDBWorkerPoolCustomDefaulter{}
	if err := d.Default(context.Background(), pool); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pool.Spec.Autoscaling.MinReplicas == nil || *pool.Spec.Autoscaling.MinReplicas != 1 {
		t.Error("expected minReplicas to be defaulted to 1")
	}
	if pool.Spec.Autoscaling.TargetCPUUtilization == nil || *pool.Spec.Autoscaling.TargetCPUUtilization != 70 {
		t.Error("expected targetCPUUtilization to be defaulted to 70")
	}
}

// --- PeerDBWorkerPool Validator ---

func TestPeerDBWorkerPoolValidator_ValidCreate(t *testing.T) {
	pool := &PeerDBWorkerPool{
		Spec: PeerDBWorkerPoolSpec{
			ClusterRef: "test",
			Replicas:   int32Ptr(2),
		},
	}

	v := &PeerDBWorkerPoolCustomValidator{} // Client nil is fine for basic validation
	_, err := v.ValidateCreate(context.Background(), pool)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPeerDBWorkerPoolValidator_MissingClusterRef(t *testing.T) {
	pool := &PeerDBWorkerPool{
		Spec: PeerDBWorkerPoolSpec{},
	}

	v := &PeerDBWorkerPoolCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), pool)
	if err == nil {
		t.Fatal("expected error for missing clusterRef")
	}
}

func TestPeerDBWorkerPoolValidator_MinGreaterThanMax(t *testing.T) {
	pool := &PeerDBWorkerPool{
		Spec: PeerDBWorkerPoolSpec{
			ClusterRef: "test",
			Autoscaling: &AutoscalingSpec{
				Enabled:     true,
				MinReplicas: int32Ptr(10),
				MaxReplicas: 5,
			},
		},
	}

	v := &PeerDBWorkerPoolCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), pool)
	if err == nil {
		t.Fatal("expected error for minReplicas > maxReplicas")
	}
}

func TestPeerDBWorkerPoolValidator_ImmutableClusterRef(t *testing.T) {
	old := &PeerDBWorkerPool{Spec: PeerDBWorkerPoolSpec{ClusterRef: "old"}}
	new := &PeerDBWorkerPool{Spec: PeerDBWorkerPoolSpec{ClusterRef: "new"}}

	v := &PeerDBWorkerPoolCustomValidator{}
	_, err := v.ValidateUpdate(context.Background(), old, new)
	if err == nil {
		t.Fatal("expected error for immutable clusterRef change")
	}
}

// --- PeerDBSnapshotPool Defaulter ---

func TestPeerDBSnapshotPoolDefaulter_DefaultsReplicas(t *testing.T) {
	pool := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			ClusterRef: "test",
			Storage:    SnapshotPoolStorageSpec{Size: resource.MustParse("10Gi")},
		},
	}

	d := &PeerDBSnapshotPoolCustomDefaulter{}
	if err := d.Default(context.Background(), pool); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pool.Spec.Replicas == nil || *pool.Spec.Replicas != 1 {
		t.Error("expected replicas to be defaulted to 1")
	}
	if pool.Spec.TerminationGracePeriodSeconds == nil || *pool.Spec.TerminationGracePeriodSeconds != 600 {
		t.Error("expected terminationGracePeriodSeconds to be defaulted to 600")
	}
}

// --- PeerDBSnapshotPool Validator ---

func TestPeerDBSnapshotPoolValidator_ValidCreate(t *testing.T) {
	pool := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			ClusterRef: "test",
			Storage:    SnapshotPoolStorageSpec{Size: resource.MustParse("10Gi")},
		},
	}

	v := &PeerDBSnapshotPoolCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), pool)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPeerDBSnapshotPoolValidator_MissingClusterRef(t *testing.T) {
	pool := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			Storage: SnapshotPoolStorageSpec{Size: resource.MustParse("10Gi")},
		},
	}

	v := &PeerDBSnapshotPoolCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), pool)
	if err == nil {
		t.Fatal("expected error for missing clusterRef")
	}
}

func TestPeerDBSnapshotPoolValidator_ZeroStorageSize(t *testing.T) {
	pool := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			ClusterRef: "test",
			Storage:    SnapshotPoolStorageSpec{},
		},
	}

	v := &PeerDBSnapshotPoolCustomValidator{}
	_, err := v.ValidateCreate(context.Background(), pool)
	if err == nil {
		t.Fatal("expected error for zero storage size")
	}
}

func TestPeerDBSnapshotPoolValidator_ImmutableClusterRef(t *testing.T) {
	old := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			ClusterRef: "old",
			Storage:    SnapshotPoolStorageSpec{Size: resource.MustParse("10Gi")},
		},
	}
	new := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			ClusterRef: "new",
			Storage:    SnapshotPoolStorageSpec{Size: resource.MustParse("10Gi")},
		},
	}

	v := &PeerDBSnapshotPoolCustomValidator{}
	_, err := v.ValidateUpdate(context.Background(), old, new)
	if err == nil {
		t.Fatal("expected error for immutable clusterRef change")
	}
}

func TestPeerDBSnapshotPoolValidator_ImmutableStorageClassName(t *testing.T) {
	sc1 := "fast"
	sc2 := "slow"
	old := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			ClusterRef: "test",
			Storage:    SnapshotPoolStorageSpec{Size: resource.MustParse("10Gi"), StorageClassName: &sc1},
		},
	}
	new := &PeerDBSnapshotPool{
		Spec: PeerDBSnapshotPoolSpec{
			ClusterRef: "test",
			Storage:    SnapshotPoolStorageSpec{Size: resource.MustParse("10Gi"), StorageClassName: &sc2},
		},
	}

	v := &PeerDBSnapshotPoolCustomValidator{}
	_, err := v.ValidateUpdate(context.Background(), old, new)
	if err == nil {
		t.Fatal("expected error for immutable storageClassName change")
	}
}

// --- Version Skew ---

func TestMajorMinorFromVersion(t *testing.T) {
	tests := []struct {
		input, want string
		wantErr     bool
	}{
		{"v0.36.7", "0.36", false},
		{"v1.2.3", "1.2", false},
		{"0.36.7", "0.36", false},
		{"latest", "", true},
	}
	for _, tt := range tests {
		got, err := MajorMinorFromVersion(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("MajorMinorFromVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("MajorMinorFromVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMajorMinorFromImage(t *testing.T) {
	tests := []struct {
		input, want string
		wantErr     bool
	}{
		{"ghcr.io/peerdb-io/flow-worker:stable-v0.36.7", "0.36", false},
		{"custom:v1.2.3", "1.2", false},
		{"notagimage", "", true},
		{"img:sha256:abc123", "", true},
	}
	for _, tt := range tests {
		got, err := MajorMinorFromImage(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("MajorMinorFromImage(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("MajorMinorFromImage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateVersionSkew(t *testing.T) {
	t.Run("matching version", func(t *testing.T) {
		err := validateVersionSkew("ghcr.io/peerdb-io/flow-worker:stable-v0.36.8", "v0.36.7", nil)
		if err != nil {
			t.Errorf("expected no error for matching major.minor, got %v", err)
		}
	})

	t.Run("mismatched version", func(t *testing.T) {
		err := validateVersionSkew("ghcr.io/peerdb-io/flow-worker:stable-v0.37.0", "v0.36.7", nil)
		if err == nil {
			t.Fatal("expected error for mismatched major.minor")
		}
	})

	t.Run("empty image", func(t *testing.T) {
		err := validateVersionSkew("", "v0.36.7", nil)
		if err != nil {
			t.Errorf("expected no error for empty image, got %v", err)
		}
	})

	t.Run("unparseable image tag", func(t *testing.T) {
		err := validateVersionSkew("custom:latest", "v0.36.7", nil)
		if err != nil {
			t.Errorf("expected no error for unparseable tag, got %v", err)
		}
	})
}
