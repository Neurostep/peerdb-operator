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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var peerdbsnapshotpoollog = logf.Log.WithName("peerdbsnapshotpool-webhook")

// PeerDBSnapshotPoolCustomDefaulter defaults PeerDBSnapshotPool resources.
//
// +kubebuilder:object:generate=false
type PeerDBSnapshotPoolCustomDefaulter struct{}

// PeerDBSnapshotPoolCustomValidator validates PeerDBSnapshotPool resources.
//
// +kubebuilder:object:generate=false
type PeerDBSnapshotPoolCustomValidator struct {
	Client client.Reader
}

var _ webhook.CustomDefaulter = &PeerDBSnapshotPoolCustomDefaulter{} //nolint:staticcheck // TODO: migrate to typed Defaulter[T]
var _ webhook.CustomValidator = &PeerDBSnapshotPoolCustomValidator{} //nolint:staticcheck // TODO: migrate to typed Validator[T]

// SetupPeerDBSnapshotPoolWebhookWithManager sets up the webhook for PeerDBSnapshotPool.
func SetupPeerDBSnapshotPoolWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &PeerDBSnapshotPool{}). //nolint:staticcheck // TODO: migrate to typed Defaulter[T]/Validator[T]
									WithCustomDefaulter(&PeerDBSnapshotPoolCustomDefaulter{}).
									WithCustomValidator(&PeerDBSnapshotPoolCustomValidator{Client: mgr.GetClient()}).
									Complete()
}

// +kubebuilder:webhook:path=/mutate-peerdb-peerdb-io-v1alpha1-peerdbsnapshotpool,mutating=true,failurePolicy=fail,sideEffects=None,groups=peerdb.peerdb.io,resources=peerdbsnapshotpools,verbs=create;update,versions=v1alpha1,name=mpeerdbsnapshotpool.kb.io,admissionReviewVersions=v1

// Default implements webhook.CustomDefaulter.
func (d *PeerDBSnapshotPoolCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	pool, ok := obj.(*PeerDBSnapshotPool)
	if !ok {
		return fmt.Errorf("expected PeerDBSnapshotPool, got %T", obj)
	}

	peerdbsnapshotpoollog.Info("defaulting", "name", pool.Name)

	// Default replicas
	if pool.Spec.Replicas == nil {
		one := int32(1)
		pool.Spec.Replicas = &one
	}

	// Default termination grace period
	if pool.Spec.TerminationGracePeriodSeconds == nil {
		grace := int64(600)
		pool.Spec.TerminationGracePeriodSeconds = &grace
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-peerdb-peerdb-io-v1alpha1-peerdbsnapshotpool,mutating=false,failurePolicy=fail,sideEffects=None,groups=peerdb.peerdb.io,resources=peerdbsnapshotpools,verbs=create;update;delete,versions=v1alpha1,name=vpeerdbsnapshotpool.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.CustomValidator.
func (v *PeerDBSnapshotPoolCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pool, ok := obj.(*PeerDBSnapshotPool)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBSnapshotPool, got %T", obj)
	}

	peerdbsnapshotpoollog.Info("validating create", "name", pool.Name)

	allErrs := validatePeerDBSnapshotPool(pool)

	// Version skew check: pinned image must match cluster major.minor.
	if pool.Spec.Image != "" && pool.Spec.ClusterRef != "" && v.Client != nil {
		cluster := &PeerDBCluster{}
		if err := v.Client.Get(ctx, types.NamespacedName{Name: pool.Spec.ClusterRef, Namespace: pool.Namespace}, cluster); err == nil {
			if skewErr := validateVersionSkew(pool.Spec.Image, cluster.Spec.Version, field.NewPath("spec", "image")); skewErr != nil {
				allErrs = append(allErrs, skewErr)
			}
		}
	}

	return nil, allErrs.ToAggregate()
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *PeerDBSnapshotPoolCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newPool, ok := newObj.(*PeerDBSnapshotPool)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBSnapshotPool, got %T", newObj)
	}
	oldPool, ok := oldObj.(*PeerDBSnapshotPool)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBSnapshotPool, got %T", oldObj)
	}

	peerdbsnapshotpoollog.Info("validating update", "name", newPool.Name)

	allErrs := validatePeerDBSnapshotPool(newPool)

	// clusterRef is immutable
	if oldPool.Spec.ClusterRef != newPool.Spec.ClusterRef {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "clusterRef"),
			"field is immutable",
		))
	}

	// storageClassName is immutable (VolumeClaimTemplates constraint)
	oldSC := oldPool.Spec.Storage.StorageClassName
	newSC := newPool.Spec.Storage.StorageClassName
	if (oldSC == nil) != (newSC == nil) || (oldSC != nil && newSC != nil && *oldSC != *newSC) {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "storage", "storageClassName"),
			"field is immutable (StatefulSet volumeClaimTemplates cannot be updated)",
		))
	}

	// Version skew check on image change.
	if newPool.Spec.Image != "" && newPool.Spec.ClusterRef != "" && v.Client != nil {
		cluster := &PeerDBCluster{}
		if err := v.Client.Get(ctx, types.NamespacedName{Name: newPool.Spec.ClusterRef, Namespace: newPool.Namespace}, cluster); err == nil {
			if skewErr := validateVersionSkew(newPool.Spec.Image, cluster.Spec.Version, field.NewPath("spec", "image")); skewErr != nil {
				allErrs = append(allErrs, skewErr)
			}
		}
	}

	return nil, allErrs.ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator.
func (v *PeerDBSnapshotPoolCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validatePeerDBSnapshotPool(pool *PeerDBSnapshotPool) field.ErrorList {
	var allErrs field.ErrorList

	if pool.Spec.ClusterRef == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "clusterRef"),
			"clusterRef is required",
		))
	}

	// Storage size must be positive
	if pool.Spec.Storage.Size.IsZero() {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "storage", "size"),
			"storage size must be specified and non-zero",
		))
	}

	return allErrs
}
