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

var peerdbworkerpoollog = logf.Log.WithName("peerdbworkerpool-webhook")

// PeerDBWorkerPoolCustomDefaulter defaults PeerDBWorkerPool resources.
//
// +kubebuilder:object:generate=false
type PeerDBWorkerPoolCustomDefaulter struct{}

// PeerDBWorkerPoolCustomValidator validates PeerDBWorkerPool resources.
//
// +kubebuilder:object:generate=false
type PeerDBWorkerPoolCustomValidator struct {
	Client client.Reader
}

var _ webhook.CustomDefaulter = &PeerDBWorkerPoolCustomDefaulter{} //nolint:staticcheck // TODO: migrate to typed Defaulter[T]
var _ webhook.CustomValidator = &PeerDBWorkerPoolCustomValidator{} //nolint:staticcheck // TODO: migrate to typed Validator[T]

// SetupPeerDBWorkerPoolWebhookWithManager sets up the webhook for PeerDBWorkerPool.
func SetupPeerDBWorkerPoolWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &PeerDBWorkerPool{}). //nolint:staticcheck // TODO: migrate to typed Defaulter[T]/Validator[T]
									WithCustomDefaulter(&PeerDBWorkerPoolCustomDefaulter{}).
									WithCustomValidator(&PeerDBWorkerPoolCustomValidator{Client: mgr.GetClient()}).
									Complete()
}

// +kubebuilder:webhook:path=/mutate-peerdb-peerdb-io-v1alpha1-peerdbworkerpool,mutating=true,failurePolicy=fail,sideEffects=None,groups=peerdb.peerdb.io,resources=peerdbworkerpools,verbs=create;update,versions=v1alpha1,name=mpeerdbworkerpool.kb.io,admissionReviewVersions=v1

// Default implements webhook.CustomDefaulter.
func (d *PeerDBWorkerPoolCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	pool, ok := obj.(*PeerDBWorkerPool)
	if !ok {
		return fmt.Errorf("expected PeerDBWorkerPool, got %T", obj)
	}

	peerdbworkerpoollog.Info("defaulting", "name", pool.Name)

	// Default replicas
	if pool.Spec.Replicas == nil {
		two := int32(2)
		pool.Spec.Replicas = &two
	}

	// Default autoscaling
	if pool.Spec.Autoscaling != nil && pool.Spec.Autoscaling.Enabled {
		if pool.Spec.Autoscaling.MinReplicas == nil {
			one := int32(1)
			pool.Spec.Autoscaling.MinReplicas = &one
		}
		if pool.Spec.Autoscaling.TargetCPUUtilization == nil {
			seventy := int32(70)
			pool.Spec.Autoscaling.TargetCPUUtilization = &seventy
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-peerdb-peerdb-io-v1alpha1-peerdbworkerpool,mutating=false,failurePolicy=fail,sideEffects=None,groups=peerdb.peerdb.io,resources=peerdbworkerpools,verbs=create;update;delete,versions=v1alpha1,name=vpeerdbworkerpool.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.CustomValidator.
func (v *PeerDBWorkerPoolCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pool, ok := obj.(*PeerDBWorkerPool)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBWorkerPool, got %T", obj)
	}

	peerdbworkerpoollog.Info("validating create", "name", pool.Name)

	allErrs := validatePeerDBWorkerPool(pool)

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
func (v *PeerDBWorkerPoolCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newPool, ok := newObj.(*PeerDBWorkerPool)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBWorkerPool, got %T", newObj)
	}
	oldPool, ok := oldObj.(*PeerDBWorkerPool)
	if !ok {
		return nil, fmt.Errorf("expected PeerDBWorkerPool, got %T", oldObj)
	}

	peerdbworkerpoollog.Info("validating update", "name", newPool.Name)

	allErrs := validatePeerDBWorkerPool(newPool)

	// clusterRef is immutable
	if oldPool.Spec.ClusterRef != newPool.Spec.ClusterRef {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "clusterRef"),
			"field is immutable",
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
func (v *PeerDBWorkerPoolCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validatePeerDBWorkerPool(pool *PeerDBWorkerPool) field.ErrorList {
	var allErrs field.ErrorList

	if pool.Spec.ClusterRef == "" {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec", "clusterRef"),
			"clusterRef is required",
		))
	}

	// Validate autoscaling min <= max (belt-and-suspenders alongside CEL)
	if pool.Spec.Autoscaling != nil && pool.Spec.Autoscaling.Enabled {
		if pool.Spec.Autoscaling.MinReplicas != nil && *pool.Spec.Autoscaling.MinReplicas > pool.Spec.Autoscaling.MaxReplicas {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec", "autoscaling", "minReplicas"),
				*pool.Spec.Autoscaling.MinReplicas,
				"minReplicas must be less than or equal to maxReplicas",
			))
		}
	}

	return allErrs
}

// validateVersionSkew checks that a pinned image's major.minor matches the cluster version.
func validateVersionSkew(image, clusterVersion string, fldPath *field.Path) *field.Error {
	if image == "" {
		return nil
	}
	clusterMM, clusterErr := MajorMinorFromVersion(clusterVersion)
	if clusterErr != nil {
		return nil //nolint:nilerr // can't validate if cluster version is unparseable
	}
	imageMM, imageErr := MajorMinorFromImage(image)
	if imageErr != nil {
		return nil //nolint:nilerr // can't validate digest-only or non-semver tags
	}
	if clusterMM != imageMM {
		return field.Invalid(fldPath, image,
			fmt.Sprintf("image major.minor %q does not match cluster version major.minor %q", imageMM, clusterMM))
	}
	return nil
}
