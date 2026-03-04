//go:build e2e
// +build e2e

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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Neurostep/peerdb-operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "peerdb-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "peerdb-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "peerdb-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "peerdb-operator-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=peerdb-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput()
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
			}
			Eventually(verifyMetricsAvailable, 2*time.Minute).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})

	Context("PeerDB resources", Ordered, func() {
		const testNs = "peerdb-e2e"

		BeforeAll(func() {
			By("creating the test namespace")
			cmd := exec.Command("kubectl", "create", "ns", testNs)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")
		})

		AfterAll(func() {
			By("removing the test namespace")
			cmd := exec.Command("kubectl", "delete", "ns", testNs, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})

		It("should set CatalogReady=False when catalog secret is missing", func() {
			By("creating a PeerDBCluster without the required secret")
			clusterYAML := `apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBCluster
metadata:
  name: test-no-secret
  namespace: ` + testNs + `
spec:
  version: "v0.36.7"
  dependencies:
    catalog:
      host: "catalog.example.com"
      port: 5432
      database: "peerdb"
      user: "peerdb"
      passwordSecretRef:
        name: missing-secret
        key: password
      sslMode: "disable"
    temporal:
      address: "temporal.example.com:7233"
      namespace: "default"
  init:
    temporalNamespaceRegistration:
      enabled: false
    temporalSearchAttributes:
      enabled: false`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(clusterYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying CatalogReady=False and Ready=False")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "peerdbcluster", "test-no-secret",
					"-n", testNs, "-o", "jsonpath={.status.conditions}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(`"type":"CatalogReady"`))
				g.Expect(output).To(ContainSubstring(`"status":"False"`))
				g.Expect(output).To(ContainSubstring(`"reason":"SecretNotFound"`))
			}).Should(Succeed())

			By("verifying no downstream resources were created")
			cmd = exec.Command("kubectl", "get", "configmap", "test-no-secret-config",
				"-n", testNs, "--no-headers")
			_, err = utils.Run(cmd)
			Expect(err).To(HaveOccurred(), "ConfigMap should not exist when secret is missing")

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "peerdbcluster", "test-no-secret",
				"-n", testNs, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})

		It("should create all expected resources for a PeerDBCluster", func() {
			By("creating the catalog password secret")
			secretYAML := `apiVersion: v1
kind: Secret
metadata:
  name: e2e-catalog-password
  namespace: ` + testNs + `
stringData:
  password: test-password`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(secretYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating the PeerDBCluster")
			clusterYAML := `apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBCluster
metadata:
  name: e2e-cluster
  namespace: ` + testNs + `
spec:
  version: "v0.36.7"
  dependencies:
    catalog:
      host: "catalog.example.com"
      port: 5432
      database: "peerdb"
      user: "peerdb"
      passwordSecretRef:
        name: e2e-catalog-password
        key: password
      sslMode: "disable"
    temporal:
      address: "temporal.example.com:7233"
      namespace: "default"
  init:
    temporalNamespaceRegistration:
      enabled: false
    temporalSearchAttributes:
      enabled: false`
			cmd = exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(clusterYAML)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying ConfigMap is created with expected keys")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", "e2e-cluster-config",
					"-n", testNs, "-o", "jsonpath={.data.PEERDB_CATALOG_HOST}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("catalog.example.com"))
			}).Should(Succeed())

			By("verifying ServiceAccount is created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "serviceaccount", "e2e-cluster",
					"-n", testNs, "--no-headers")
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())

			By("verifying Flow API Deployment and Service are created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "e2e-cluster-flow-api",
					"-n", testNs, "--no-headers")
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())

			cmd = exec.Command("kubectl", "get", "service", "e2e-cluster-flow-api",
				"-n", testNs, "--no-headers")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying PeerDB Server Deployment and Service are created")
			cmd = exec.Command("kubectl", "get", "deployment", "e2e-cluster-server",
				"-n", testNs, "--no-headers")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "get", "service", "e2e-cluster-server",
				"-n", testNs, "--no-headers")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying UI Deployment and Service are created")
			cmd = exec.Command("kubectl", "get", "deployment", "e2e-cluster-ui",
				"-n", testNs, "--no-headers")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "get", "service", "e2e-cluster-ui",
				"-n", testNs, "--no-headers")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying CatalogReady and TemporalReady conditions are True")
			cmd = exec.Command("kubectl", "get", "peerdbcluster", "e2e-cluster",
				"-n", testNs, "-o", "jsonpath={.status.conditions}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`"type":"CatalogReady"`))
			Expect(output).To(ContainSubstring(`"type":"TemporalReady"`))

			By("verifying ownerReferences on ConfigMap point to PeerDBCluster")
			cmd = exec.Command("kubectl", "get", "configmap", "e2e-cluster-config",
				"-n", testNs, "-o", "jsonpath={.metadata.ownerReferences[0].kind}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("PeerDBCluster"))

			By("verifying observedGeneration is set")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "peerdbcluster", "e2e-cluster",
					"-n", testNs, "-o", "jsonpath={.status.observedGeneration}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"))
			}).Should(Succeed())
		})

		It("should create a worker Deployment for PeerDBWorkerPool", func() {
			By("creating the PeerDBWorkerPool")
			poolYAML := `apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBWorkerPool
metadata:
  name: e2e-workers
  namespace: ` + testNs + `
spec:
  clusterRef: "e2e-cluster"
  replicas: 1
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "200m"
      memory: "256Mi"`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(poolYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying worker Deployment is created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "e2e-workers",
					"-n", testNs, "--no-headers")
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())

			By("verifying ownerReferences point to PeerDBWorkerPool")
			cmd = exec.Command("kubectl", "get", "deployment", "e2e-workers",
				"-n", testNs, "-o", "jsonpath={.metadata.ownerReferences[0].kind}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("PeerDBWorkerPool"))

			By("verifying worker pool status has Ready condition")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "peerdbworkerpool", "e2e-workers",
					"-n", testNs, "-o", "jsonpath={.status.conditions}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(`"type":"Ready"`))
			}).Should(Succeed())
		})

		It("should create a StatefulSet and headless Service for PeerDBSnapshotPool", func() {
			By("creating the PeerDBSnapshotPool")
			poolYAML := `apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBSnapshotPool
metadata:
  name: e2e-snapshot
  namespace: ` + testNs + `
spec:
  clusterRef: "e2e-cluster"
  replicas: 1
  storage:
    size: "1Gi"
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "200m"
      memory: "256Mi"`
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(poolYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying snapshot StatefulSet is created")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "statefulset", "e2e-snapshot",
					"-n", testNs, "--no-headers")
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())

			By("verifying headless Service is created")
			cmd = exec.Command("kubectl", "get", "service", "e2e-snapshot",
				"-n", testNs, "-o", "jsonpath={.spec.clusterIP}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("None"))

			By("verifying ownerReferences point to PeerDBSnapshotPool")
			cmd = exec.Command("kubectl", "get", "statefulset", "e2e-snapshot",
				"-n", testNs, "-o", "jsonpath={.metadata.ownerReferences[0].kind}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("PeerDBSnapshotPool"))

			By("verifying snapshot pool status has Ready condition")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "peerdbsnapshotpool", "e2e-snapshot",
					"-n", testNs, "-o", "jsonpath={.status.conditions}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring(`"type":"Ready"`))
			}).Should(Succeed())
		})

		It("should clean up owned resources when PeerDBCluster is deleted", func() {
			By("deleting the PeerDBWorkerPool")
			cmd := exec.Command("kubectl", "delete", "peerdbworkerpool", "e2e-workers",
				"-n", testNs, "--ignore-not-found")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deleting the PeerDBSnapshotPool")
			cmd = exec.Command("kubectl", "delete", "peerdbsnapshotpool", "e2e-snapshot",
				"-n", testNs, "--ignore-not-found")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deleting the PeerDBCluster")
			cmd = exec.Command("kubectl", "delete", "peerdbcluster", "e2e-cluster",
				"-n", testNs)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying owned resources are garbage collected")
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "configmap", "e2e-cluster-config",
					"-n", testNs, "--no-headers")
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "ConfigMap should be garbage collected")
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployment", "e2e-cluster-flow-api",
					"-n", testNs, "--no-headers")
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred(), "Flow API Deployment should be garbage collected")
			}).Should(Succeed())
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	return utils.Run(cmd)
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
