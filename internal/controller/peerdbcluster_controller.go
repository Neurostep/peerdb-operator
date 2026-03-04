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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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

// PeerDBClusterReconciler reconciles a PeerDBCluster object
type PeerDBClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=peerdb.peerdb.io,resources=peerdbclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=peerdb.peerdb.io,resources=peerdbclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=peerdb.peerdb.io,resources=peerdbclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *PeerDBClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	cluster := &peerdbv1alpha1.PeerDBCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Set Reconciling condition at the start.
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               peerdbv1alpha1.ConditionReconciling,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
		Reason:             peerdbv1alpha1.ReasonReconciling,
		Message:            "Reconciliation in progress",
	})

	if cluster.Spec.Paused {
		log.Info("PeerDBCluster is paused, skipping reconciliation")
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonPaused,
			Message:            "Cluster reconciliation is paused",
		})
		r.Recorder.Event(cluster, corev1.EventTypeNormal, peerdbv1alpha1.ReasonPaused, "Cluster reconciliation is paused")
		peerdbmetrics.ClusterReady.WithLabelValues(cluster.Name, cluster.Namespace).Set(0)
		cluster.Status.ObservedGeneration = cluster.Generation
		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if backup fencing is active.
	backupInProgress := cluster.Annotations[peerdbv1alpha1.AnnotationBackupInProgress] != ""
	if backupInProgress {
		log.Info("Backup in progress, fencing destructive operations")
		r.Recorder.Event(cluster, corev1.EventTypeNormal, peerdbv1alpha1.ReasonBackupInProgress, "Backup in progress: destructive operations are fenced")
	}

	// Validate dependency secret refs exist.
	catalogSecretRef := cluster.Spec.Dependencies.Catalog.PasswordSecretRef
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: catalogSecretRef.Name, Namespace: cluster.Namespace}, secret); err != nil {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionCatalogReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonSecretNotFound,
			Message:            fmt.Sprintf("Catalog password secret %q not found", catalogSecretRef.Name),
		})
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonDependencyNotReady,
			Message:            "Catalog dependency is not ready",
		})
		r.Recorder.Eventf(cluster, corev1.EventTypeWarning, peerdbv1alpha1.ReasonSecretNotFound, "Catalog password secret %q not found", catalogSecretRef.Name)
		peerdbmetrics.ReconcileErrorsTotal.WithLabelValues("peerdbcluster").Inc()
		peerdbmetrics.ClusterReady.WithLabelValues(cluster.Name, cluster.Namespace).Set(0)
		cluster.Status.ObservedGeneration = cluster.Generation
		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               peerdbv1alpha1.ConditionCatalogReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
		Reason:             peerdbv1alpha1.ReasonSecretFound,
		Message:            "Catalog password secret is available",
	})
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               peerdbv1alpha1.ConditionTemporalReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
		Reason:             peerdbv1alpha1.ReasonConfigured,
		Message:            fmt.Sprintf("Temporal configured at %s", cluster.Spec.Dependencies.Temporal.Address),
	})

	// Compute config hash for rollout annotations.
	desiredConfigMap := resources.BuildConfigMap(cluster)
	secretRVs := map[string]string{
		catalogSecretRef.Name: secret.ResourceVersion,
	}
	configHash := resources.ComputeConfigHash(desiredConfigMap.Data, secretRVs)

	// Reconcile ServiceAccount.
	if sa := resources.BuildServiceAccount(cluster); sa != nil {
		if err := r.reconcileServiceAccount(ctx, cluster, sa); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile ConfigMap.
	if err := r.reconcileConfigMap(ctx, cluster, desiredConfigMap); err != nil {
		return ctrl.Result{}, err
	}

	// Initialize upgrade status if nil (first reconcile or pre-upgrade-orchestration cluster).
	if cluster.Status.Upgrade == nil {
		cluster.Status.Upgrade = &peerdbv1alpha1.UpgradeStatus{
			ToVersion: cluster.Spec.Version,
			Phase:     peerdbv1alpha1.UpgradePhaseComplete,
		}
	}

	if backupInProgress {
		// Skip upgrades and mutations during backup fencing.
		log.Info("Skipping upgrade reconciliation during backup")
	} else {
		// Detect version upgrade request.
		upgradeInProgress := cluster.Status.Upgrade.Phase != peerdbv1alpha1.UpgradePhaseComplete
		versionChanged := cluster.Spec.Version != cluster.Status.Upgrade.ToVersion

		if versionChanged && !upgradeInProgress {
			// New upgrade requested.
			now := metav1.Now()
			cluster.Status.Upgrade = &peerdbv1alpha1.UpgradeStatus{
				FromVersion: cluster.Status.Upgrade.ToVersion,
				ToVersion:   cluster.Spec.Version,
				Phase:       peerdbv1alpha1.UpgradePhaseWaiting,
				StartedAt:   &now,
				Message:     "Upgrade requested",
			}
			upgradeInProgress = true
			r.Recorder.Eventf(cluster, corev1.EventTypeNormal, peerdbv1alpha1.ReasonUpgradeInProgress,
				"Version upgrade requested: %s → %s", cluster.Status.Upgrade.FromVersion, cluster.Spec.Version)
			log.Info("Version upgrade requested",
				"from", cluster.Status.Upgrade.FromVersion,
				"to", cluster.Spec.Version)
		}

		if upgradeInProgress {
			result, err := r.reconcileUpgrade(ctx, cluster, configHash)
			if err != nil {
				return result, err
			}
			// After upgrade reconciliation, continue to status update below.
			if result.RequeueAfter > 0 || result.Requeue {
				// Update status and requeue.
				cluster.Status.ObservedGeneration = cluster.Generation
				if statusErr := r.updateStatus(ctx, cluster); statusErr != nil {
					if apierrors.IsConflict(statusErr) {
						return ctrl.Result{Requeue: true}, nil
					}
					return ctrl.Result{}, statusErr
				}
				return result, nil
			}
		}
	}

	// Standard reconciliation (no upgrade in progress, or upgrade complete).
	// Reconcile init jobs.
	initReady := true
	if !backupInProgress {
		if err := r.reconcileInitJobs(ctx, cluster, &initReady); err != nil {
			return ctrl.Result{}, err
		}
	}
	if initReady {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionInitialized,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonJobsCompleted,
			Message:            "All init jobs completed successfully",
		})
	} else {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionInitialized,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonJobsPending,
			Message:            "Init jobs have not completed yet",
		})
	}

	// Reconcile components: Flow API, PeerDB Server, UI.
	componentsReady := true

	if backupInProgress {
		// Read-only status check during backup fencing.
		for _, depName := range []string{
			fmt.Sprintf("%s-flow-api", cluster.Name),
			fmt.Sprintf("%s-server", cluster.Name),
			fmt.Sprintf("%s-ui", cluster.Name),
		} {
			dep := &appsv1.Deployment{}
			if err := r.Get(ctx, types.NamespacedName{Name: depName, Namespace: cluster.Namespace}, dep); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				componentsReady = false
			} else if dep.Status.ReadyReplicas < dep.Status.Replicas {
				componentsReady = false
			}
		}
	} else {
		if err := r.reconcileDeploymentAndService(ctx, cluster,
			resources.BuildFlowAPIDeployment(cluster, configHash),
			resources.BuildFlowAPIService(cluster),
			&componentsReady,
		); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileDeploymentAndService(ctx, cluster,
			resources.BuildPeerDBServerDeployment(cluster, configHash),
			resources.BuildPeerDBServerService(cluster),
			&componentsReady,
		); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileDeploymentAndService(ctx, cluster,
			resources.BuildUIDeployment(cluster, configHash),
			resources.BuildUIService(cluster),
			&componentsReady,
		); err != nil {
			return ctrl.Result{}, err
		}
	}

	if componentsReady {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionComponentsReady,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonAllReady,
			Message:            "All components are ready",
		})
	} else {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionComponentsReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonComponentsNotReady,
			Message:            "Some components are not ready yet",
		})
	}

	// Set overall Ready condition.
	overallReady := initReady && componentsReady
	if overallReady {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionReady,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonClusterReady,
			Message:            "PeerDB cluster is ready",
		})
		r.Recorder.Event(cluster, corev1.EventTypeNormal, peerdbv1alpha1.ReasonClusterReady, "PeerDB cluster is ready")
		peerdbmetrics.ClusterReady.WithLabelValues(cluster.Name, cluster.Namespace).Set(1)
	} else {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonClusterNotReady,
			Message:            "PeerDB cluster is not fully ready",
		})
		peerdbmetrics.ClusterReady.WithLabelValues(cluster.Name, cluster.Namespace).Set(0)
	}

	// Set BackupSafe condition.
	upgradeActive := cluster.Status.Upgrade != nil && cluster.Status.Upgrade.Phase != peerdbv1alpha1.UpgradePhaseComplete
	if backupInProgress {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionBackupSafe,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonBackupInProgress,
			Message:            "Backup is in progress, destructive operations are fenced",
		})
	} else if upgradeActive || !componentsReady {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionBackupSafe,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonBackupUnsafe,
			Message:            "Cluster is not safe for backup: upgrade or rollout in progress",
		})
	} else {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionBackupSafe,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonBackupSafe,
			Message:            "Cluster is safe for backup",
		})
	}

	// Clear Reconciling condition.
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               peerdbv1alpha1.ConditionReconciling,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: cluster.Generation,
		Reason:             peerdbv1alpha1.ReasonReconcileComplete,
		Message:            "Reconciliation complete",
	})

	// Update endpoints.
	cluster.Status.Endpoints = &peerdbv1alpha1.EndpointStatus{
		ServerAddress:  fmt.Sprintf("%s-server.%s.svc.cluster.local:9900", cluster.Name, cluster.Namespace),
		UIAddress:      fmt.Sprintf("%s-ui.%s.svc.cluster.local:3000", cluster.Name, cluster.Namespace),
		FlowAPIAddress: fmt.Sprintf("%s-flow-api.%s.svc.cluster.local:8112", cluster.Name, cluster.Namespace),
	}
	cluster.Status.ObservedGeneration = cluster.Generation

	if statusErr := r.updateStatus(ctx, cluster); statusErr != nil {
		if apierrors.IsConflict(statusErr) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, statusErr
	}

	if !overallReady {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileUpgrade drives the ordered upgrade state machine.
func (r *PeerDBClusterReconciler) reconcileUpgrade(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, configHash string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	upgrade := cluster.Status.Upgrade

	// Set UpgradeInProgress condition.
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               peerdbv1alpha1.ConditionUpgradeInProgress,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
		Reason:             peerdbv1alpha1.ReasonUpgradeInProgress,
		Message:            fmt.Sprintf("Upgrading %s → %s (phase: %s)", upgrade.FromVersion, upgrade.ToVersion, upgrade.Phase),
	})

	switch upgrade.Phase {
	case peerdbv1alpha1.UpgradePhaseWaiting:
		return r.upgradePhaseWaiting(ctx, cluster)

	case peerdbv1alpha1.UpgradePhaseBlocked:
		return r.upgradePhaseBlocked(ctx, cluster)

	case peerdbv1alpha1.UpgradePhaseConfig:
		// Config/secrets are always reconciled before we get here, so advance immediately.
		upgrade.Phase = peerdbv1alpha1.UpgradePhaseInitJobs
		upgrade.Message = "Reconciling init jobs"
		log.Info("Upgrade advancing to InitJobs phase")
		return ctrl.Result{Requeue: true}, nil

	case peerdbv1alpha1.UpgradePhaseInitJobs:
		return r.upgradePhaseInitJobs(ctx, cluster)

	case peerdbv1alpha1.UpgradePhaseFlowAPI:
		return r.upgradePhaseComponent(ctx, cluster, configHash,
			resources.BuildFlowAPIDeployment(cluster, configHash),
			resources.BuildFlowAPIService(cluster),
			"Flow API",
			peerdbv1alpha1.UpgradePhaseServer,
		)

	case peerdbv1alpha1.UpgradePhaseServer:
		return r.upgradePhaseComponent(ctx, cluster, configHash,
			resources.BuildPeerDBServerDeployment(cluster, configHash),
			resources.BuildPeerDBServerService(cluster),
			"PeerDB Server",
			peerdbv1alpha1.UpgradePhaseUI,
		)

	case peerdbv1alpha1.UpgradePhaseUI:
		return r.upgradePhaseComponent(ctx, cluster, configHash,
			resources.BuildUIDeployment(cluster, configHash),
			resources.BuildUIService(cluster),
			"UI",
			peerdbv1alpha1.UpgradePhaseComplete,
		)
	}

	return ctrl.Result{}, nil
}

func (r *PeerDBClusterReconciler) upgradePhaseWaiting(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	upgrade := cluster.Status.Upgrade

	// Check upgrade policy.
	policy := peerdbv1alpha1.UpgradePolicyAutomatic
	if cluster.Spec.UpgradePolicy != nil {
		policy = *cluster.Spec.UpgradePolicy
	}
	if policy == peerdbv1alpha1.UpgradePolicyManual {
		upgrade.Message = "Upgrade pending: set spec.upgradePolicy to Automatic to proceed"
		log.Info("Upgrade waiting for manual approval")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check maintenance window.
	if cluster.Spec.MaintenanceWindow != nil {
		if !isInMaintenanceWindow(cluster.Spec.MaintenanceWindow) {
			upgrade.Phase = peerdbv1alpha1.UpgradePhaseWaiting
			upgrade.Message = "Waiting for maintenance window"
			r.Recorder.Event(cluster, corev1.EventTypeNormal, peerdbv1alpha1.ReasonMaintenanceWindow, "Upgrade waiting for maintenance window")
			log.Info("Upgrade waiting for maintenance window")
			return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
		}
	}

	// Check dependency health (safety gate).
	catalogCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionCatalogReady)
	temporalCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionTemporalReady)
	if (catalogCond != nil && catalogCond.Status == metav1.ConditionFalse) ||
		(temporalCond != nil && temporalCond.Status == metav1.ConditionFalse) {
		upgrade.Phase = peerdbv1alpha1.UpgradePhaseBlocked
		upgrade.Message = "Blocked: dependencies unhealthy"
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionDegraded,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonDegraded,
			Message:            "Upgrade blocked: dependencies unhealthy",
		})
		r.Recorder.Event(cluster, corev1.EventTypeWarning, peerdbv1alpha1.ReasonUpgradeBlocked, "Upgrade blocked: dependencies unhealthy")
		log.Info("Upgrade blocked due to unhealthy dependencies")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Proceed to config phase.
	upgrade.Phase = peerdbv1alpha1.UpgradePhaseConfig
	upgrade.Message = "Reconciling configuration"
	log.Info("Upgrade advancing to Config phase")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PeerDBClusterReconciler) upgradePhaseBlocked(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	upgrade := cluster.Status.Upgrade

	// Re-check dependency health.
	catalogCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionCatalogReady)
	temporalCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionTemporalReady)
	if (catalogCond == nil || catalogCond.Status == metav1.ConditionTrue) &&
		(temporalCond == nil || temporalCond.Status == metav1.ConditionTrue) {
		// Dependencies are healthy now, move back to Waiting to re-evaluate policy/window.
		upgrade.Phase = peerdbv1alpha1.UpgradePhaseWaiting
		upgrade.Message = "Dependencies healthy, re-evaluating upgrade"
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionDegraded,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonReconcileComplete,
			Message:            "Dependencies healthy",
		})
		log.Info("Upgrade unblocked, re-evaluating")
		return ctrl.Result{Requeue: true}, nil
	}

	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               peerdbv1alpha1.ConditionDegraded,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
		Reason:             peerdbv1alpha1.ReasonDegraded,
		Message:            "Upgrade blocked: dependencies unhealthy",
	})
	upgrade.Message = "Blocked: dependencies unhealthy (CatalogReady/TemporalReady)"
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *PeerDBClusterReconciler) upgradePhaseInitJobs(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	upgrade := cluster.Status.Upgrade

	initReady := true
	if err := r.reconcileInitJobs(ctx, cluster, &initReady); err != nil {
		return ctrl.Result{}, err
	}

	if !initReady {
		// Check if any job failed.
		initFailed := false
		if cluster.Spec.Init == nil || isInitJobEnabled(cluster.Spec.Init.TemporalNamespaceRegistration) {
			jobName := fmt.Sprintf("%s-temporal-ns-register-%s", cluster.Name, resources.SanitizeVersion(cluster.Spec.Version))
			if failed, _ := r.isJobFailed(ctx, cluster.Namespace, jobName); failed {
				initFailed = true
				upgrade.Message = fmt.Sprintf("Blocked: init job %q failed", jobName)
			}
		}
		if !initFailed && (cluster.Spec.Init == nil || isInitJobEnabled(cluster.Spec.Init.TemporalSearchAttributes)) {
			jobName := fmt.Sprintf("%s-temporal-search-attr-%s", cluster.Name, resources.SanitizeVersion(cluster.Spec.Version))
			if failed, _ := r.isJobFailed(ctx, cluster.Namespace, jobName); failed {
				initFailed = true
				upgrade.Message = fmt.Sprintf("Blocked: init job %q failed", jobName)
			}
		}

		if initFailed {
			upgrade.Phase = peerdbv1alpha1.UpgradePhaseBlocked
			r.Recorder.Event(cluster, corev1.EventTypeWarning, peerdbv1alpha1.ReasonUpgradeBlocked, upgrade.Message)
			meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
				Type:               peerdbv1alpha1.ConditionInitialized,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: cluster.Generation,
				Reason:             peerdbv1alpha1.ReasonJobFailed,
				Message:            upgrade.Message,
			})
		} else {
			upgrade.Message = "Waiting for init jobs to complete"
			meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
				Type:               peerdbv1alpha1.ConditionInitialized,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: cluster.Generation,
				Reason:             peerdbv1alpha1.ReasonJobsPending,
				Message:            "Init jobs have not completed yet",
			})
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               peerdbv1alpha1.ConditionInitialized,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
		Reason:             peerdbv1alpha1.ReasonJobsCompleted,
		Message:            "All init jobs completed successfully",
	})

	upgrade.Phase = peerdbv1alpha1.UpgradePhaseFlowAPI
	upgrade.Message = "Rolling out Flow API"
	log.Info("Upgrade advancing to FlowAPI phase")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PeerDBClusterReconciler) upgradePhaseComponent(
	ctx context.Context,
	cluster *peerdbv1alpha1.PeerDBCluster,
	configHash string,
	deployment *appsv1.Deployment,
	service *corev1.Service,
	componentName string,
	nextPhase peerdbv1alpha1.UpgradePhase,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	upgrade := cluster.Status.Upgrade

	componentsReady := true
	if err := r.reconcileDeploymentAndService(ctx, cluster, deployment, service, &componentsReady); err != nil {
		return ctrl.Result{}, err
	}

	// Check if the deployment has fully rolled out the new version.
	existing := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, existing); err != nil {
		return ctrl.Result{}, err
	}

	desiredReplicas := int32(1)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}

	if !deploymentRolledOut(existing, desiredReplicas) {
		upgrade.Message = fmt.Sprintf("Rolling out %s (%d/%d updated, %d/%d available)",
			componentName,
			existing.Status.UpdatedReplicas, desiredReplicas,
			existing.Status.AvailableReplicas, desiredReplicas)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Component is fully rolled out.
	if nextPhase == peerdbv1alpha1.UpgradePhaseComplete {
		// All components upgraded.
		upgrade.Phase = peerdbv1alpha1.UpgradePhaseComplete
		upgrade.Message = fmt.Sprintf("Upgrade complete: %s → %s", upgrade.FromVersion, upgrade.ToVersion)
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               peerdbv1alpha1.ConditionUpgradeInProgress,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             peerdbv1alpha1.ReasonUpgradeComplete,
			Message:            upgrade.Message,
		})
		r.Recorder.Eventf(cluster, corev1.EventTypeNormal, peerdbv1alpha1.ReasonUpgradeComplete,
			"Version upgrade complete: %s → %s", upgrade.FromVersion, upgrade.ToVersion)
		log.Info("Upgrade complete", "from", upgrade.FromVersion, "to", upgrade.ToVersion)
		// Return empty result to continue to standard reconciliation.
		return ctrl.Result{}, nil
	}

	upgrade.Phase = nextPhase
	upgrade.Message = fmt.Sprintf("Rolling out %s", nextPhase)
	log.Info("Upgrade advancing", "nextPhase", nextPhase)
	return ctrl.Result{Requeue: true}, nil
}

func (r *PeerDBClusterReconciler) isJobFailed(ctx context.Context, namespace, name string) (bool, error) {
	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, job); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true, nil
		}
	}
	return false, nil
}

func (r *PeerDBClusterReconciler) updateStatus(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster) error {
	log := logf.FromContext(ctx)
	latest := &peerdbv1alpha1.PeerDBCluster{}
	if err := r.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, latest); err != nil {
		return err
	}
	if equality.Semantic.DeepEqual(latest.Status, cluster.Status) {
		log.V(1).Info("status unchanged; skipping update")
		return nil
	}
	latest.Status = cluster.Status
	return r.Status().Update(ctx, latest)
}

func (r *PeerDBClusterReconciler) reconcileServiceAccount(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, desired *corev1.ServiceAccount) error {
	if err := controllerutil.SetControllerReference(cluster, desired, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) ||
		!equality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		existing.Annotations = desired.Annotations
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}
	return nil
}

func (r *PeerDBClusterReconciler) reconcileConfigMap(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, desired *corev1.ConfigMap) error {
	if err := controllerutil.SetControllerReference(cluster, desired, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(existing.Data, desired.Data) {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		return r.Update(ctx, existing)
	}
	return nil
}

func (r *PeerDBClusterReconciler) reconcileInitJobs(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, initReady *bool) error {
	initEnabled := func(js *peerdbv1alpha1.InitJobSpec) bool {
		return js == nil || js.Enabled == nil || *js.Enabled
	}

	// Namespace registration job.
	nsEnabled := cluster.Spec.Init == nil || initEnabled(cluster.Spec.Init.TemporalNamespaceRegistration)
	if nsEnabled {
		if err := r.reconcileJob(ctx, cluster, resources.BuildNamespaceRegistrationJob(cluster), initReady); err != nil {
			return err
		}
	}

	// Search attribute job.
	saEnabled := cluster.Spec.Init == nil || initEnabled(cluster.Spec.Init.TemporalSearchAttributes)
	if saEnabled {
		if err := r.reconcileJob(ctx, cluster, resources.BuildSearchAttributeJob(cluster), initReady); err != nil {
			return err
		}
	}

	return nil
}

func (r *PeerDBClusterReconciler) reconcileJob(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, desired *batchv1.Job, ready *bool) error {
	if err := controllerutil.SetControllerReference(cluster, desired, r.Scheme); err != nil {
		return err
	}

	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		*ready = false
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Check if job completed.
	for _, c := range existing.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return nil
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			*ready = false
			return nil
		}
	}

	*ready = false
	return nil
}

func (r *PeerDBClusterReconciler) reconcileDeploymentAndService(
	ctx context.Context,
	cluster *peerdbv1alpha1.PeerDBCluster,
	deployment *appsv1.Deployment,
	service *corev1.Service,
	componentsReady *bool,
) error {
	if err := r.reconcileDeployment(ctx, cluster, deployment, componentsReady); err != nil {
		return err
	}
	return r.reconcileService(ctx, cluster, service)
}

func (r *PeerDBClusterReconciler) reconcileDeployment(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, desired *appsv1.Deployment, ready *bool) error {
	if err := controllerutil.SetControllerReference(cluster, desired, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		*ready = false
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Check readiness.
	if existing.Status.ReadyReplicas < existing.Status.Replicas {
		*ready = false
	}

	// Snapshot the live object before mutation.
	before := existing.DeepCopy()

	// Mutate only the fields we manage on the live object, preserving
	// all Kubernetes-defaulted fields (imagePullPolicy, terminationMessagePath,
	// probe defaults, etc.) to avoid false diffs that trigger rolling restarts.
	existing.Labels = desired.Labels
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template.Labels = desired.Spec.Template.Labels
	existing.Spec.Template.Spec.ServiceAccountName = desired.Spec.Template.Spec.ServiceAccountName

	// Manage config-hash annotation on the pod template.
	if existing.Spec.Template.Annotations == nil {
		existing.Spec.Template.Annotations = map[string]string{}
	}
	if desired.Spec.Template.Annotations != nil {
		if v, ok := desired.Spec.Template.Annotations[resources.AnnotationConfigHash]; ok {
			existing.Spec.Template.Annotations[resources.AnnotationConfigHash] = v
		}
	}

	if len(existing.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		dc := desired.Spec.Template.Spec.Containers[0]
		existing.Spec.Template.Spec.Containers[0].Image = dc.Image
		existing.Spec.Template.Spec.Containers[0].Env = dc.Env
		existing.Spec.Template.Spec.Containers[0].EnvFrom = dc.EnvFrom
		existing.Spec.Template.Spec.Containers[0].Ports = dc.Ports
		existing.Spec.Template.Spec.Containers[0].Resources = dc.Resources
	}

	// Only update if something actually changed.
	if !equality.Semantic.DeepEqual(before.Spec, existing.Spec) {
		if err := r.Update(ctx, existing); err != nil {
			if apierrors.IsConflict(err) {
				return nil // will be retried on next reconcile
			}
			return err
		}
	}

	return nil
}

func (r *PeerDBClusterReconciler) reconcileService(ctx context.Context, cluster *peerdbv1alpha1.PeerDBCluster, desired *corev1.Service) error {
	if err := controllerutil.SetControllerReference(cluster, desired, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update mutable fields only if changed.
	if !equality.Semantic.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) ||
		!equality.Semantic.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) ||
		!equality.Semantic.DeepEqual(existing.Labels, desired.Labels) ||
		!equality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) {
		existing.Spec.Ports = desired.Spec.Ports
		existing.Spec.Selector = desired.Spec.Selector
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		return r.Update(ctx, existing)
	}
	return nil
}

// deploymentRolledOut checks if a deployment has fully rolled out to the desired state.
func deploymentRolledOut(dep *appsv1.Deployment, desiredReplicas int32) bool {
	return dep.Status.ObservedGeneration >= dep.Generation &&
		dep.Status.UpdatedReplicas == desiredReplicas &&
		dep.Status.AvailableReplicas == desiredReplicas
}

// isInMaintenanceWindow checks if the current time falls within the configured maintenance window.
func isInMaintenanceWindow(window *peerdbv1alpha1.MaintenanceWindow) bool {
	loc := time.UTC
	if window.TimeZone != nil {
		if l, err := time.LoadLocation(*window.TimeZone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)
	nowMinutes := now.Hour()*60 + now.Minute()

	start, err := parseTimeOfDay(window.Start)
	if err != nil {
		return true // if we can't parse, allow the upgrade
	}
	end, err := parseTimeOfDay(window.End)
	if err != nil {
		return true
	}

	if start <= end {
		return nowMinutes >= start && nowMinutes < end
	}
	// Window crosses midnight (e.g., 23:00-03:00).
	return nowMinutes >= start || nowMinutes < end
}

// parseTimeOfDay parses "HH:MM" into minutes since midnight.
func parseTimeOfDay(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

// isInitJobEnabled checks if an init job spec is enabled.
func isInitJobEnabled(js *peerdbv1alpha1.InitJobSpec) bool {
	return js == nil || js.Enabled == nil || *js.Enabled
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeerDBClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&peerdbv1alpha1.PeerDBCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&batchv1.Job{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](1*time.Second, 60*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
		}).
		Named("peerdbcluster").
		Complete(r)
}
