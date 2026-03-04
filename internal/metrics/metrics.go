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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ClusterReady is a gauge that indicates whether a PeerDBCluster is ready.
	// Labels: name, namespace.
	ClusterReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "peerdb_cluster_ready",
			Help: "Whether a PeerDBCluster is ready (1) or not (0).",
		},
		[]string{"name", "namespace"},
	)

	// ReconcileErrorsTotal is a counter tracking reconciliation errors.
	// Labels: controller.
	ReconcileErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "peerdb_reconcile_errors_total",
			Help: "Total number of reconciliation errors.",
		},
		[]string{"controller"},
	)

	// WorkerPoolsTotal is a gauge tracking the total number of worker pool replicas.
	// Labels: name, namespace.
	WorkerPoolsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "peerdb_worker_pools_total",
			Help: "Total number of worker pool replicas.",
		},
		[]string{"name", "namespace"},
	)

	// SnapshotPoolsTotal is a gauge tracking the total number of snapshot pool replicas.
	// Labels: name, namespace.
	SnapshotPoolsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "peerdb_snapshot_pools_total",
			Help: "Total number of snapshot pool replicas.",
		},
		[]string{"name", "namespace"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		ClusterReady,
		ReconcileErrorsTotal,
		WorkerPoolsTotal,
		SnapshotPoolsTotal,
	)
}
