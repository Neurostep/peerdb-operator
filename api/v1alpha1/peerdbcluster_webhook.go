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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var peerdbclusterlog = logf.Log.WithName("peerdbcluster-webhook")

// PeerDBClusterCustomDefaulter defaults PeerDBCluster resources.
//
// +kubebuilder:object:generate=false
type PeerDBClusterCustomDefaulter struct{}

// PeerDBClusterCustomValidator validates PeerDBCluster resources.
//
// +kubebuilder:object:generate=false
type PeerDBClusterCustomValidator struct{}

var _ webhook.CustomDefaulter = &PeerDBClusterCustomDefaulter{} //nolint:staticcheck // TODO: migrate to typed Defaulter[T]
var _ webhook.CustomValidator = &PeerDBClusterCustomValidator{} //nolint:staticcheck // TODO: migrate to typed Validator[T]

// SetupPeerDBClusterWebhookWithManager sets up the webhook for PeerDBCluster.
func SetupPeerDBClusterWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &PeerDBCluster{}). //nolint:staticcheck // TODO: migrate to typed Defaulter[T]/Validator[T]
								WithCustomDefaulter(&PeerDBClusterCustomDefaulter{}).
								WithCustomValidator(&PeerDBClusterCustomValidator{}).
								Complete()
}

// +kubebuilder:webhook:path=/mutate-peerdb-peerdb-io-v1alpha1-peerdbcluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=peerdb.peerdb.io,resources=peerdbclusters,verbs=create;update,versions=v1alpha1,name=mpeerdbcluster.kb.io,admissionReviewVersions=v1

// Default implements webhook.CustomDefaulter.
func (d *PeerDBClusterCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	cluster, ok := obj.(*PeerDBCluster)
	if !ok {
		return fmt.Errorf("expected PeerDBCluster, got %T", obj)
	}

	peerdbclusterlog.Info("defaulting", "name", cluster.Name)

	// Default catalog port
	if cluster.Spec.Dependencies.Catalog.Port == 0 {
		cluster.Spec.Dependencies.Catalog.Port = 5432
	}

	// Default catalog SSL mode
	if cluster.Spec.Dependencies.Catalog.SSLMode == "" {
		cluster.Spec.Dependencies.Catalog.SSLMode = "require"
	}

	// Default service account
	if cluster.Spec.ServiceAccount == nil {
		cluster.Spec.ServiceAccount = &ServiceAccountConfig{Create: true}
	}

	// Default component replicas
	if cluster.Spec.Components != nil {
		if cluster.Spec.Components.FlowAPI != nil && cluster.Spec.Components.FlowAPI.Replicas == nil {
			one := int32(1)
			cluster.Spec.Components.FlowAPI.Replicas = &one
		}
		if cluster.Spec.Components.PeerDBServer != nil && cluster.Spec.Components.PeerDBServer.Replicas == nil {
			one := int32(1)
			cluster.Spec.Components.PeerDBServer.Replicas = &one
		}
		if cluster.Spec.Components.UI != nil && cluster.Spec.Components.UI.Replicas == nil {
			one := int32(1)
			cluster.Spec.Components.UI.Replicas = &one
		}
		if cluster.Spec.Components.AuthProxy != nil && cluster.Spec.Components.AuthProxy.Replicas == nil {
			one := int32(1)
			cluster.Spec.Components.AuthProxy.Replicas = &one
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-peerdb-peerdb-io-v1alpha1-peerdbcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=peerdb.peerdb.io,resources=peerdbclusters,verbs=create;update;delete,versions=v1alpha1,name=vpeerdbcluster.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.CustomValidator.
func (v *PeerDBClusterCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	cluster, ok := obj.(*PeerDBCluster)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBCluster, got %T", obj)
	}

	peerdbclusterlog.Info("validating create", "name", cluster.Name)

	return nil, validatePeerDBCluster(cluster).ToAggregate()
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *PeerDBClusterCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newCluster, ok := newObj.(*PeerDBCluster)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBCluster, got %T", newObj)
	}
	oldCluster, ok := oldObj.(*PeerDBCluster)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBCluster, got %T", oldObj)
	}

	peerdbclusterlog.Info("validating update", "name", newCluster.Name)

	allErrs := validatePeerDBCluster(newCluster)

	// Immutable field check: dependencies.catalog.host cannot change once set
	if oldCluster.Spec.Dependencies.Catalog.Host != newCluster.Spec.Dependencies.Catalog.Host {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "dependencies", "catalog", "host"),
			"field is immutable",
		))
	}

	return nil, allErrs.ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator.
func (v *PeerDBClusterCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validatePeerDBCluster(cluster *PeerDBCluster) field.ErrorList {
	var allErrs field.ErrorList

	// Version must not be empty (already enforced by kubebuilder marker, but double-check)
	if cluster.Spec.Version == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "version"),
			"version is required",
		))
	}

	// Catalog host must be set
	if cluster.Spec.Dependencies.Catalog.Host == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "dependencies", "catalog", "host"),
			"catalog host is required",
		))
	}

	// Catalog password secret ref must be set
	if cluster.Spec.Dependencies.Catalog.PasswordSecretRef.Name == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "dependencies", "catalog", "passwordSecretRef", "name"),
			"catalog password secret name is required",
		))
	}

	// Temporal address must be set
	if cluster.Spec.Dependencies.Temporal.Address == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "dependencies", "temporal", "address"),
			"temporal address is required",
		))
	}

	// Temporal namespace must be set
	if cluster.Spec.Dependencies.Temporal.Namespace == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "dependencies", "temporal", "namespace"),
			"temporal namespace is required",
		))
	}

	return allErrs
}
