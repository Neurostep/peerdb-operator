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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	peerdbv1alpha1 "github.com/Neurostep/peerdb-operator/api/v1alpha1"
)

var _ = Describe("PeerDBCluster Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		peerdbcluster := &peerdbv1alpha1.PeerDBCluster{}

		BeforeEach(func() {
			By("creating the catalog password secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "catalog-password",
					Namespace: "default",
				},
				StringData: map[string]string{
					"password": "test-password",
				},
			}
			err := k8sClient.Create(ctx, secret)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the custom resource for the Kind PeerDBCluster")
			err = k8sClient.Get(ctx, typeNamespacedName, peerdbcluster)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &peerdbv1alpha1.PeerDBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: peerdbv1alpha1.PeerDBClusterSpec{
						Version: "v0.36.7",
						Dependencies: peerdbv1alpha1.DependenciesSpec{
							Catalog: peerdbv1alpha1.CatalogSpec{
								Host:     "catalog.example.com",
								Database: "peerdb",
								User:     "peerdb",
								PasswordSecretRef: peerdbv1alpha1.SecretKeySelector{
									Name: "catalog-password",
									Key:  "password",
								},
							},
							Temporal: peerdbv1alpha1.TemporalSpec{
								Address:   "temporal.example.com:7233",
								Namespace: "peerdb",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &peerdbv1alpha1.PeerDBCluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PeerDBCluster")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create ConfigMap with correct data", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			cm := &corev1.ConfigMap{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-config",
				Namespace: "default",
			}, cm)
			Expect(err).NotTo(HaveOccurred())

			Expect(cm.Data).To(HaveKeyWithValue("PEERDB_CATALOG_HOST", "catalog.example.com"))
			Expect(cm.Data).To(HaveKeyWithValue("PEERDB_CATALOG_DATABASE", "peerdb"))
			Expect(cm.Data).To(HaveKeyWithValue("PEERDB_CATALOG_USER", "peerdb"))
			Expect(cm.Data).To(HaveKeyWithValue("PEERDB_CATALOG_PORT", "5432"))
			Expect(cm.Data).To(HaveKeyWithValue("PEERDB_CATALOG_SSL_MODE", "require"))
			Expect(cm.Data).To(HaveKeyWithValue("TEMPORAL_HOST_PORT", "temporal.example.com:7233"))
			Expect(cm.Data).To(HaveKeyWithValue("TEMPORAL_NAMESPACE", "peerdb"))
		})

		It("should create ServiceAccount", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			sa := &corev1.ServiceAccount{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}, sa)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create Flow API Deployment and Service", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-flow-api",
				Namespace: "default",
			}, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deploy.Spec.Template.Spec.Containers[0].Ports).To(ContainElements(
				HaveField("ContainerPort", int32(8112)),
				HaveField("ContainerPort", int32(8113)),
			))

			svc := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-flow-api",
				Namespace: "default",
			}, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Spec.Ports).To(ContainElements(
				HaveField("Port", int32(8112)),
				HaveField("Port", int32(8113)),
			))
		})

		It("should create PeerDB Server Deployment and Service", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-server",
				Namespace: "default",
			}, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deploy.Spec.Template.Spec.Containers[0].Ports).To(ContainElement(
				HaveField("ContainerPort", int32(9900)),
			))

			svc := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-server",
				Namespace: "default",
			}, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Spec.Ports).To(ContainElement(
				HaveField("Port", int32(9900)),
			))
		})

		It("should create UI Deployment and Service", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			deploy := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-ui",
				Namespace: "default",
			}, deploy)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deploy.Spec.Template.Spec.Containers[0].Ports).To(ContainElement(
				HaveField("ContainerPort", int32(3000)),
			))

			svc := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-ui",
				Namespace: "default",
			}, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Spec.Ports).To(ContainElement(
				HaveField("Port", int32(3000)),
			))
		})

		It("should create init jobs", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			nsJob := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-temporal-ns-register-v0-36-7",
				Namespace: "default",
			}, nsJob)
			Expect(err).NotTo(HaveOccurred())

			saJob := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-temporal-search-attr-v0-36-7",
				Namespace: "default",
			}, saJob)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete failed init jobs for automatic retry", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}

			By("running initial reconciliation to create jobs")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("marking the namespace-register job as failed")
			nsJobName := resourceName + "-temporal-ns-register-v0-36-7"
			nsJob := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      nsJobName,
				Namespace: "default",
			}, nsJob)
			Expect(err).NotTo(HaveOccurred())

			now := metav1.Now()
			nsJob.Status.StartTime = &now
			nsJob.Status.Conditions = append(nsJob.Status.Conditions,
				batchv1.JobCondition{
					Type:   batchv1.JobFailureTarget,
					Status: corev1.ConditionTrue,
				},
				batchv1.JobCondition{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			)
			Expect(k8sClient.Status().Update(ctx, nsJob)).To(Succeed())

			By("reconciling again — the failed job should be deleted")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      nsJobName,
				Namespace: "default",
			}, &batchv1.Job{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected failed job to be deleted")

			By("verifying Ready condition is False (job not yet complete)")
			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())

			readyCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
		})

		It("should set Ready condition to False when components are not ready", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())

			readyCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
		})
	})

	Context("When reconciling a paused cluster", func() {
		const resourceName = "test-resource-paused"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the catalog password secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "catalog-password-paused",
					Namespace: "default",
				},
				StringData: map[string]string{
					"password": "test-password",
				},
			}
			err := k8sClient.Create(ctx, secret)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating a paused PeerDBCluster")
			resource := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && apierrors.IsNotFound(err) {
				resource = &peerdbv1alpha1.PeerDBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: peerdbv1alpha1.PeerDBClusterSpec{
						Version: "v0.36.7",
						Paused:  true,
						Dependencies: peerdbv1alpha1.DependenciesSpec{
							Catalog: peerdbv1alpha1.CatalogSpec{
								Host:     "catalog.example.com",
								Database: "peerdb",
								User:     "peerdb",
								PasswordSecretRef: peerdbv1alpha1.SecretKeySelector{
									Name: "catalog-password-paused",
									Key:  "password",
								},
							},
							Temporal: peerdbv1alpha1.TemporalSpec{
								Address:   "temporal.example.com:7233",
								Namespace: "peerdb",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &peerdbv1alpha1.PeerDBCluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the paused PeerDBCluster")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should handle paused cluster", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())

			readyCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCond.Reason).To(Equal("Paused"))
		})
	})

	Context("When reconciling a cluster with maintenance mode during upgrade", func() {
		const resourceName = "test-resource-maintenance"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the catalog password secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "catalog-password-maintenance",
					Namespace: "default",
				},
				StringData: map[string]string{
					"password": "test-password",
				},
			}
			err := k8sClient.Create(ctx, secret)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating a PeerDBCluster with maintenance mode")
			resource := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && apierrors.IsNotFound(err) {
				resource = &peerdbv1alpha1.PeerDBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: peerdbv1alpha1.PeerDBClusterSpec{
						Version:     "v0.36.7",
						Maintenance: &peerdbv1alpha1.MaintenanceSpec{},
						Dependencies: peerdbv1alpha1.DependenciesSpec{
							Catalog: peerdbv1alpha1.CatalogSpec{
								Host:     "catalog.example.com",
								Database: "peerdb",
								User:     "peerdb",
								PasswordSecretRef: peerdbv1alpha1.SecretKeySelector{
									Name: "catalog-password-maintenance",
									Key:  "password",
								},
							},
							Temporal: peerdbv1alpha1.TemporalSpec{
								Address:   "temporal.example.com:7233",
								Namespace: "peerdb",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &peerdbv1alpha1.PeerDBCluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the maintenance PeerDBCluster")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should enter StartMaintenance phase and create maintenance job during upgrade", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}

			By("running initial reconciliation to create resources at v0.36.7")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("updating version to trigger an upgrade")
			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			cluster.Spec.Version = "v0.36.8"
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			By("reconciling to detect version change — should enter Waiting phase")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("reconciling again — should advance to StartMaintenance and create the job")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("verifying the start maintenance job was created")
			job := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-maintenance-start-v0-36-8",
				Namespace: "default",
			}, job)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElements("/root/peer-flow", "maintenance", "start"))

			By("verifying the upgrade status shows StartMaintenance phase")
			cluster = &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.Status.Upgrade).NotTo(BeNil())
			Expect(cluster.Status.Upgrade.Phase).To(Equal(peerdbv1alpha1.UpgradePhaseStartMaintenance))

			By("verifying MaintenanceMode condition is set")
			maintCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionMaintenanceMode)
			Expect(maintCond).NotTo(BeNil())
			Expect(maintCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(maintCond.Reason).To(Equal(peerdbv1alpha1.ReasonMaintenanceStarting))
		})

		It("should advance past StartMaintenance when job completes", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}

			By("running initial reconciliation")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("triggering upgrade to v0.36.8")
			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			cluster.Spec.Version = "v0.36.8"
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			By("reconciling through Waiting → StartMaintenance")
			for i := 0; i < 3; i++ {
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			By("marking the start maintenance job as complete")
			job := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-maintenance-start-v0-36-8",
				Namespace: "default",
			}, job)
			Expect(err).NotTo(HaveOccurred())
			now := metav1.Now()
			job.Status.StartTime = &now
			job.Status.CompletionTime = &now
			job.Status.Conditions = append(job.Status.Conditions,
				batchv1.JobCondition{
					Type:   batchv1.JobSuccessCriteriaMet,
					Status: corev1.ConditionTrue,
				},
				batchv1.JobCondition{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			)
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			By("reconciling — should advance past StartMaintenance to Config")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("verifying upgrade phase advanced past StartMaintenance")
			cluster = &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.Status.Upgrade).NotTo(BeNil())
			// After StartMaintenance completes, it should advance to Config (then immediately to InitJobs)
			phase := cluster.Status.Upgrade.Phase
			Expect(phase).To(BeElementOf(
				peerdbv1alpha1.UpgradePhaseConfig,
				peerdbv1alpha1.UpgradePhaseInitJobs,
				peerdbv1alpha1.UpgradePhaseFlowAPI,
			))

			By("verifying MaintenanceMode condition shows Active")
			maintCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionMaintenanceMode)
			Expect(maintCond).NotTo(BeNil())
			Expect(maintCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(maintCond.Reason).To(Equal(peerdbv1alpha1.ReasonMaintenanceActive))
		})

		It("should create EndMaintenance job with MaintenanceEnding reason after all components upgrade", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}

			By("running initial reconciliation")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("triggering upgrade to v0.36.8")
			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			cluster.Spec.Version = "v0.36.8"
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			By("reconciling through Waiting → StartMaintenance → job creation")
			for i := 0; i < 3; i++ {
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			By("completing the start maintenance job")
			startJob := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-maintenance-start-v0-36-8",
				Namespace: "default",
			}, startJob)
			Expect(err).NotTo(HaveOccurred())
			now := metav1.Now()
			startJob.Status.StartTime = &now
			startJob.Status.CompletionTime = &now
			startJob.Status.Conditions = append(startJob.Status.Conditions,
				batchv1.JobCondition{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
				batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			)
			Expect(k8sClient.Status().Update(ctx, startJob)).To(Succeed())

			By("reconciling through Config → InitJobs → FlowAPI → Server → UI → EndMaintenance")
			// Helper: simulate deployment rollout (envtest has no real pods).
			simulateDeploymentRollout := func(name string) {
				dep := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, dep); err != nil {
					return
				}
				replicas := int32(1)
				if dep.Spec.Replicas != nil {
					replicas = *dep.Spec.Replicas
				}
				dep.Status.ObservedGeneration = dep.Generation
				dep.Status.Replicas = replicas
				dep.Status.UpdatedReplicas = replicas
				dep.Status.AvailableReplicas = replicas
				dep.Status.ReadyReplicas = replicas
				_ = k8sClient.Status().Update(ctx, dep)
			}
			// Helper: mark init job as complete.
			completeJob := func(name string) {
				j := &batchv1.Job{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, j); err != nil {
					return
				}
				if len(j.Status.Conditions) > 0 {
					return // already has conditions
				}
				t := metav1.Now()
				j.Status.StartTime = &t
				j.Status.CompletionTime = &t
				j.Status.Conditions = []batchv1.JobCondition{
					{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
					{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
				}
				_ = k8sClient.Status().Update(ctx, j)
			}

			for i := 0; i < 20; i++ {
				_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				completeJob(resourceName + "-temporal-ns-register-v0-36-8")
				completeJob(resourceName + "-temporal-search-attr-v0-36-8")
				simulateDeploymentRollout(resourceName + "-flow-api")
				simulateDeploymentRollout(resourceName + "-server")
				simulateDeploymentRollout(resourceName + "-ui")
			}

			By("verifying the end maintenance job was created")
			endJob := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-maintenance-end-v0-36-8",
				Namespace: "default",
			}, endJob)
			Expect(err).NotTo(HaveOccurred())
			Expect(endJob.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(endJob.Spec.Template.Spec.Containers[0].Command).To(ContainElements("/root/peer-flow", "maintenance", "end"))

			By("verifying MaintenanceMode condition shows MaintenanceEnding reason")
			cluster = &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			maintCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionMaintenanceMode)
			Expect(maintCond).NotTo(BeNil())
			Expect(maintCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(maintCond.Reason).To(Equal(peerdbv1alpha1.ReasonMaintenanceEnding))

			By("completing the end maintenance job")
			endJob.Status.StartTime = &now
			endJob.Status.CompletionTime = &now
			endJob.Status.Conditions = append(endJob.Status.Conditions,
				batchv1.JobCondition{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
				batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			)
			Expect(k8sClient.Status().Update(ctx, endJob)).To(Succeed())

			By("reconciling — should complete the upgrade")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("verifying upgrade is complete")
			cluster = &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.Status.Upgrade).NotTo(BeNil())
			Expect(cluster.Status.Upgrade.Phase).To(Equal(peerdbv1alpha1.UpgradePhaseComplete))

			By("verifying MaintenanceMode condition is False with MaintenanceComplete reason")
			maintCond = meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionMaintenanceMode)
			Expect(maintCond).NotTo(BeNil())
			Expect(maintCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(maintCond.Reason).To(Equal(peerdbv1alpha1.ReasonMaintenanceComplete))
		})

		It("should not enter maintenance phases when maintenance is not configured", func() {
			By("creating a cluster without maintenance")
			noMaintName := "test-no-maint"
			noMaintNN := types.NamespacedName{Name: noMaintName, Namespace: "default"}

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "catalog-password-no-maint",
					Namespace: "default",
				},
				StringData: map[string]string{
					"password": "test-password",
				},
			}
			err := k8sClient.Create(ctx, secret)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			resource := &peerdbv1alpha1.PeerDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noMaintName,
					Namespace: "default",
				},
				Spec: peerdbv1alpha1.PeerDBClusterSpec{
					Version: "v0.36.7",
					Dependencies: peerdbv1alpha1.DependenciesSpec{
						Catalog: peerdbv1alpha1.CatalogSpec{
							Host:     "catalog.example.com",
							Database: "peerdb",
							User:     "peerdb",
							PasswordSecretRef: peerdbv1alpha1.SecretKeySelector{
								Name: "catalog-password-no-maint",
								Key:  "password",
							},
						},
						Temporal: peerdbv1alpha1.TemporalSpec{
							Address:   "temporal.example.com:7233",
							Namespace: "peerdb",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}

			By("running initial reconciliation")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: noMaintNN,
			})
			Expect(err).NotTo(HaveOccurred())

			By("triggering upgrade")
			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, noMaintNN, cluster)
			Expect(err).NotTo(HaveOccurred())
			cluster.Spec.Version = "v0.36.8"
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			By("reconciling through the upgrade")
			for i := 0; i < 3; i++ {
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: noMaintNN,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			By("verifying no maintenance job was created")
			job := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      noMaintName + "-maintenance-start-v0-36-8",
				Namespace: "default",
			}, job)
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "maintenance job should not exist without spec.maintenance")

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should delete and retry failed maintenance jobs", func() {
			controllerReconciler := &PeerDBClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(10),
			}

			By("running initial reconciliation")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("triggering upgrade to v0.36.9")
			cluster := &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			cluster.Spec.Version = "v0.36.9"
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			By("reconciling to create the maintenance job")
			for i := 0; i < 3; i++ {
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			By("marking the maintenance job as failed")
			jobName := resourceName + "-maintenance-start-v0-36-9"
			job := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: "default",
			}, job)
			Expect(err).NotTo(HaveOccurred())

			now := metav1.Now()
			job.Status.StartTime = &now
			job.Status.Conditions = append(job.Status.Conditions,
				batchv1.JobCondition{
					Type:   batchv1.JobFailureTarget,
					Status: corev1.ConditionTrue,
				},
				batchv1.JobCondition{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			)
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			By("reconciling — should delete the failed job")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      jobName,
				Namespace: "default",
			}, &batchv1.Job{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected failed maintenance job to be deleted")

			By("verifying Degraded condition is set")
			cluster = &peerdbv1alpha1.PeerDBCluster{}
			err = k8sClient.Get(ctx, typeNamespacedName, cluster)
			Expect(err).NotTo(HaveOccurred())
			degradedCond := meta.FindStatusCondition(cluster.Status.Conditions, peerdbv1alpha1.ConditionDegraded)
			Expect(degradedCond).NotTo(BeNil())
			Expect(degradedCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(degradedCond.Reason).To(Equal(peerdbv1alpha1.ReasonMaintenanceFailed))
		})
	})
})
