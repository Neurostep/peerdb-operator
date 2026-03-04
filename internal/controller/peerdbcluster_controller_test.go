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
})
