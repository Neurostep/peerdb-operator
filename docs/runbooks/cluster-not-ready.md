# Runbook: Cluster Stuck NotReady

## Symptoms

The PeerDBCluster resource shows `Ready=False` in its status:

```
$ kubectl get peerdbcluster
NAME     READY   VERSION   AGE
peerdb   False   v0.36.7   15m
```

## Diagnostic Steps

### 1. Check All Conditions

Start by inspecting the full set of conditions to identify which subsystem is unhealthy:

```bash
kubectl get peerdbcluster <name> -o jsonpath='{.status.conditions}' | jq .
```

Or in a human-readable table:

```bash
kubectl get peerdbcluster <name> -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'
```

### 2. CatalogReady=False — Missing or Incorrect Catalog Secret

**Cause:** The Secret referenced by `spec.dependencies.catalog.passwordSecretRef` does not exist or does not contain the expected key.

**Diagnosis:**

```bash
# Check which secret is expected
kubectl get peerdbcluster <name> -o jsonpath='{.spec.dependencies.catalog.passwordSecretRef}'

# Verify the secret exists
kubectl get secret <secret-name>

# Verify the key exists in the secret
kubectl get secret <secret-name> -o jsonpath='{.data.<key>}'
```

**Resolution:**

- Create the Secret if it does not exist:
  ```bash
  kubectl create secret generic <secret-name> --from-literal=<key>=<password>
  ```
- If the Secret exists but the key is wrong, update the Secret or fix `spec.dependencies.catalog.passwordSecretRef.key`.
- Verify the catalog database is reachable from the cluster (check hostname, port, network policies).

### 3. TemporalReady=False — Temporal Address Misconfigured

**Cause:** The Temporal frontend service is unreachable or misconfigured.

**Diagnosis:**

```bash
# Check Temporal config
kubectl get peerdbcluster <name> -o jsonpath='{.spec.dependencies.temporal}'

# Test connectivity from within the cluster
kubectl run -it --rm debug --image=busybox -- nc -zv <temporal-host> <temporal-port>
```

**Resolution:**

- Verify `spec.dependencies.temporal.address` points to a valid `host:port`.
- Check DNS resolution — ensure the Temporal service is discoverable from the namespace.
- If Temporal uses TLS, verify the `tlsSecretRef` Secret exists and contains valid `tls.crt` and `tls.key` entries.
- Check network policies that might block egress to the Temporal service.

### 4. Initialized=False — Init Jobs Not Completed

**Cause:** The Temporal namespace registration or search attribute jobs have not completed.

**Diagnosis:**

```bash
kubectl get jobs -l app.kubernetes.io/instance=<cluster-name>
kubectl logs job/<cluster-name>-temporal-ns-register
kubectl logs job/<cluster-name>-temporal-search-attr
```

**Resolution:**

See the dedicated [Init Jobs Failing](init-jobs-failing.md) runbook.

### 5. ComponentsReady=False — Deployments Not Ready

**Cause:** One or more PeerDB control plane Deployments (Flow API, PeerDB Server, UI) are not ready.

**Diagnosis:**

```bash
# Check deployment status
kubectl get deployments -l app.kubernetes.io/instance=<cluster-name>

# Check pods
kubectl get pods -l app.kubernetes.io/instance=<cluster-name>

# Describe a failing pod for events
kubectl describe pod <pod-name>

# Check pod logs
kubectl logs <pod-name>
```

**Common causes:**

- Image pull failures (wrong image tag, missing `imagePullSecrets`)
- Insufficient cluster resources (CPU/memory)
- CrashLoopBackOff due to incorrect configuration (catalog unreachable, Temporal unreachable)
- Readiness probe failures

**Resolution:**

- Fix image references or add `spec.imagePullSecrets`.
- Ensure cluster has sufficient resources or adjust component `resources` in the spec.
- Verify external dependencies (catalog DB, Temporal) are reachable from the pods.

### 6. Degraded=True — Partial Failure

**Cause:** The cluster is running but some components are unhealthy.

**Diagnosis:**

```bash
kubectl get peerdbcluster <name> -o jsonpath='{.status.conditions}' | jq '.[] | select(.type=="Degraded")'
```

**Resolution:**

- Check each component individually using the steps above.
- A degraded state often means some replicas are down while others are serving traffic.
- Address the underlying component issue; the condition will clear automatically once all components are healthy.

## Checking Operator Logs

If the conditions do not provide enough information, check the operator controller logs:

```bash
# Find the operator namespace (typically the namespace where it was deployed)
kubectl logs -l app.kubernetes.io/name=peerdb-operator -n <operator-ns>

# Follow logs in real-time
kubectl logs -f -l app.kubernetes.io/name=peerdb-operator -n <operator-ns>

# Filter for a specific cluster
kubectl logs -l app.kubernetes.io/name=peerdb-operator -n <operator-ns> | grep <cluster-name>
```

## Requeue Behavior

The operator uses different requeue intervals depending on the issue:

| Situation | Requeue Interval |
|-----------|-----------------|
| Dependency not ready (missing Secret, Temporal unreachable) | 30 seconds |
| Components not yet ready (Deployments rolling out) | 10 seconds |
| Just created a resource | 5 seconds |

If a cluster has been NotReady for longer than a few minutes, the issue is unlikely to self-resolve and requires manual intervention.
