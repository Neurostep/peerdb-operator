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

package controller

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	peerdbv1alpha1 "github.com/Neurostep/peerdb-operator/api/v1alpha1"
	peerdbmetrics "github.com/Neurostep/peerdb-operator/internal/metrics"
	"github.com/Neurostep/peerdb-operator/internal/resources"
)

// PeerDBSnapshotPoolReconciler reconciles a PeerDBSnapshotPool object
type PeerDBSnapshotPoolReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=peerdb.peerdb.io,resources=peerdbsnapshotpools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=peerdb.peerdb.io,resources=peerdbsnapshotpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=peerdb.peerdb.io,resources=peerdbsnapshotpools/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *PeerDBSnapshotPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	pool := &peerdbv1alpha1.PeerDBSnapshotPool{}
	if err := r.Get(ctx, req.NamespacedName, pool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Snapshot status before mutations to avoid unnecessary API writes.
	statusSnapshot := pool.Status.DeepCopy()

	// Fetch the referenced PeerDBCluster.
	cluster := &peerdbv1alpha1.PeerDBCluster{}
	if err := r.Get(ctx, types.NamespacedName{Name: pool.Spec.ClusterRef, Namespace: pool.Namespace}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Referenced PeerDBCluster not found", "clusterRef", pool.Spec.ClusterRef)
			meta.SetStatusCondition(&pool.Status.Conditions, metav1.Condition{
				Type:               peerdbv1alpha1.ConditionReady,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: pool.Generation,
				Reason:             peerdbv1alpha1.ReasonClusterNotFound,
				Message:            fmt.Sprintf("Referenced PeerDBCluster %q not found", pool.Spec.ClusterRef),
			})
			r.Recorder.Eventf(pool, nil, corev1.EventTypeWarning, peerdbv1alpha1.ReasonClusterNotFound, "DependencyCheck", "Referenced PeerDBCluster %q not found", pool.Spec.ClusterRef)
			peerdbmetrics.ReconcileErrorsTotal.WithLabelValues("peerdbsnapshotpool").Inc()
			pool.Status.ObservedGeneration = pool.Generation
			if statusErr := r.Status().Update(ctx, pool); statusErr != nil {
				log.Error(statusErr, "Failed to update status after cluster not found")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if backup fencing is active on the referenced cluster.
	backupInProgress := cluster.Annotations[peerdbv1alpha1.AnnotationBackupInProgress] != ""

	// Reconcile headless Service.
	if err := r.reconcileService(ctx, pool); err != nil {
		return ctrl.Result{}, err
	}

	// Warn on version skew if the pool pins a custom image.
	r.checkVersionSkew(pool, cluster)

	// Compute config hash from the cluster's ConfigMap and catalog secret.
	configHash := r.computePoolConfigHash(ctx, cluster, pool.Namespace)

	// Build and reconcile the Snapshot Worker StatefulSet.
	desired := resources.BuildSnapshotWorkerStatefulSet(pool, cluster, configHash)
	if err := controllerutil.SetControllerReference(pool, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		meta.SetStatusCondition(&pool.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: pool.Generation,
			Reason:             peerdbv1alpha1.ReasonStatefulSetCreated,
			Message:            "Snapshot worker StatefulSet was created and is starting",
		})
		r.Recorder.Eventf(pool, nil, corev1.EventTypeNormal, peerdbv1alpha1.ReasonStatefulSetCreated, "Created", "Snapshot worker StatefulSet was created")
		pool.Status.ObservedGeneration = pool.Generation
		if statusErr := r.Status().Update(ctx, pool); statusErr != nil {
			log.Error(statusErr, "Failed to update status after StatefulSet creation")
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update mutable fields (skip during backup fencing).
	if !backupInProgress {
		if err := r.updateStatefulSet(ctx, existing, desired); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status from StatefulSet.
	pool.Status.Replicas = existing.Status.Replicas
	pool.Status.ReadyReplicas = existing.Status.ReadyReplicas
	pool.Status.ObservedGeneration = pool.Generation
	peerdbmetrics.SnapshotPoolsTotal.WithLabelValues(pool.Name, pool.Namespace).Set(float64(pool.Status.Replicas))

	desiredReplicas := int32(1)
	if pool.Spec.Replicas != nil {
		desiredReplicas = *pool.Spec.Replicas
	}

	ready := existing.Status.ReadyReplicas == desiredReplicas
	if desiredReplicas == 0 {
		ready = true // scale-to-zero is valid
	}

	if ready {
		meta.SetStatusCondition(&pool.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionReady,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: pool.Generation,
			Reason:             peerdbv1alpha1.ReasonStatefulSetReady,
			Message:            fmt.Sprintf("%d/%d replicas are ready", existing.Status.ReadyReplicas, desiredReplicas),
		})
		r.Recorder.Eventf(pool, nil, corev1.EventTypeNormal, peerdbv1alpha1.ReasonStatefulSetReady, "Reconciled", "%d/%d replicas are ready", existing.Status.ReadyReplicas, desiredReplicas)
	} else {
		meta.SetStatusCondition(&pool.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: pool.Generation,
			Reason:             peerdbv1alpha1.ReasonStatefulSetNotReady,
			Message:            fmt.Sprintf("%d/%d replicas are ready", existing.Status.ReadyReplicas, desiredReplicas),
		})
	}

	if !equality.Semantic.DeepEqual(*statusSnapshot, pool.Status) {
		if err := r.Status().Update(ctx, pool); err != nil {
			return ctrl.Result{}, err
		}
	}

	if !ready {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *PeerDBSnapshotPoolReconciler) reconcileService(ctx context.Context, pool *peerdbv1alpha1.PeerDBSnapshotPool) error {
	desiredSvc := resources.BuildSnapshotWorkerService(pool)
	if err := controllerutil.SetControllerReference(pool, desiredSvc, r.Scheme); err != nil {
		return err
	}
	existingSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: desiredSvc.Name, Namespace: desiredSvc.Namespace}, existingSvc); apierrors.IsNotFound(err) {
		return r.Create(ctx, desiredSvc)
	} else if err != nil {
		return err
	}
	existingSvc.Spec.Selector = desiredSvc.Spec.Selector
	existingSvc.Labels = desiredSvc.Labels
	return r.Update(ctx, existingSvc)
}

func (r *PeerDBSnapshotPoolReconciler) checkVersionSkew(pool *peerdbv1alpha1.PeerDBSnapshotPool, cluster *peerdbv1alpha1.PeerDBCluster) {
	if pool.Spec.Image == "" {
		return
	}
	clusterMM, clusterErr := peerdbv1alpha1.MajorMinorFromVersion(cluster.Spec.Version)
	imageMM, imageErr := peerdbv1alpha1.MajorMinorFromImage(pool.Spec.Image)
	if clusterErr == nil && imageErr == nil && clusterMM != imageMM {
		r.Recorder.Eventf(pool, nil, corev1.EventTypeWarning, peerdbv1alpha1.ReasonVersionSkew, "VersionSkewDetected",
			"Pinned image %q (major.minor %s) does not match cluster version %s (major.minor %s)",
			pool.Spec.Image, imageMM, cluster.Spec.Version, clusterMM)
	}
}

func (r *PeerDBSnapshotPoolReconciler) computePoolConfigHash(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, namespace string) string {
	desiredConfigMap := resources.BuildConfigMap(cluster)
	catalogSecret := &corev1.Secret{}
	secretRVs := map[string]string{}
	if err := r.Get(ctx, types.NamespacedName{Name: cluster.Spec.Dependencies.Catalog.PasswordSecretRef.Name, Namespace: namespace}, catalogSecret); err == nil {
		secretRVs[catalogSecret.Name] = catalogSecret.ResourceVersion
	}
	return resources.ComputeConfigHash(desiredConfigMap.Data, secretRVs)
}

func (r *PeerDBSnapshotPoolReconciler) updateStatefulSet(ctx context.Context, existing, desired *appsv1.StatefulSet) error {
	needsUpdate := false
	if !equality.Semantic.DeepEqual(existing.Spec.Template, desired.Spec.Template) {
		existing.Spec.Template = desired.Spec.Template
		needsUpdate = true
	}
	if !equality.Semantic.DeepEqual(existing.Spec.Replicas, desired.Spec.Replicas) {
		existing.Spec.Replicas = desired.Spec.Replicas
		needsUpdate = true
	}
	if needsUpdate {
		return r.Update(ctx, existing)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeerDBSnapshotPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&peerdbv1alpha1.PeerDBSnapshotPool{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](1*time.Second, 60*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
		}).
		Named("peerdbsnapshotpool").
		Complete(r)
}
