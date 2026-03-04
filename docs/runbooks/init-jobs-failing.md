# Runbook: Init Jobs Failing

## Symptoms

The PeerDBCluster shows `Initialized=False` with reason `JobFailed` or `JobsPending`:

```
$ kubectl get peerdbcluster <name> -o jsonpath='{.status.conditions}' | jq '.[] | select(.type=="Initialized")'
{
  "type": "Initialized",
  "status": "False",
  "reason": "JobFailed",
  "message": "Init job peerdb-temporal-ns-register failed"
}
```

## What Init Jobs Do

The operator creates two init jobs during cluster setup:

| Job Name | Purpose | Image |
|----------|---------|-------|
| `{clusterName}-temporal-ns-register` | Registers the Temporal namespace used by PeerDB using `tctl` | `temporalio/admin-tools` |
| `{clusterName}-temporal-search-attr` | Registers PeerDB-specific search attributes in Temporal | `temporalio/admin-tools` |

Both jobs connect to the Temporal frontend service specified in `spec.dependencies.temporal.address`.

## Diagnostic Steps

### 1. Check Job Status

```bash
kubectl get jobs -l app.kubernetes.io/instance=<cluster-name>
```

Example output:
```
NAME                                COMPLETIONS   DURATION   AGE
peerdb-temporal-ns-register         0/1           5m         5m
peerdb-temporal-search-attr         0/1           5m         5m
```

### 2. Check Job Logs

```bash
# Namespace registration job
kubectl logs job/<cluster-name>-temporal-ns-register

# Search attributes job
kubectl logs job/<cluster-name>-temporal-search-attr
```

If the job has multiple attempts, check individual pod logs:

```bash
kubectl get pods -l job-name=<cluster-name>-temporal-ns-register
kubectl logs <pod-name>
```

### 3. Check Job Events

```bash
kubectl describe job <cluster-name>-temporal-ns-register
```

### 4. Common Failure Causes

#### Temporal Unreachable

**Symptom:** Logs show connection refused or timeout errors.

```
rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp 10.0.0.1:7233: connect: connection refused"
```

**Resolution:**
- Verify `spec.dependencies.temporal.address` is correct (format: `host:port`).
- Test connectivity from within the cluster:
  ```bash
  kubectl run -it --rm debug --image=busybox -- nc -zv <temporal-host> <temporal-port>
  ```
- Check network policies that might block egress to Temporal.

#### DNS Resolution Failure

**Symptom:** Logs show DNS lookup errors.

```
rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp: lookup temporal-frontend.temporal.svc.cluster.local: no such host"
```

**Resolution:**
- Verify the Temporal service exists in the expected namespace.
- If Temporal is external, ensure DNS is resolvable from within the cluster.
- Check CoreDNS pods are healthy: `kubectl get pods -n kube-system -l k8s-app=kube-dns`

#### Namespace Already Exists (Not a Real Failure)

The namespace registration job is designed to be idempotent. If the Temporal namespace already exists, the job should succeed. If it fails with "namespace already exists", this may indicate an older version of the admin-tools image. Try:

```bash
# Delete the failed job — the controller will recreate it
kubectl delete job <cluster-name>-temporal-ns-register
```

#### TLS Certificate Issues

**Symptom:** Logs show TLS handshake errors.

**Resolution:**
- Verify the `spec.dependencies.temporal.tlsSecretRef` Secret exists.
- Verify the Secret contains valid `tls.crt` and `tls.key` entries.
- Ensure the certificate is trusted by the Temporal server.

## Resolution

### Retry Failed Jobs

Init jobs are **create-once** — the controller creates them but does not recreate them if they already exist. To retry a failed job:

```bash
# Delete the failed job
kubectl delete job <cluster-name>-temporal-ns-register

# The controller will recreate it on the next reconciliation loop (within ~30s)
```

Repeat for the search attributes job if needed:

```bash
kubectl delete job <cluster-name>-temporal-search-attr
```

### Disable Jobs for Pre-Configured Temporal

If your Temporal instance already has the PeerDB namespace and search attributes configured, you can skip the init jobs entirely:

```yaml
apiVersion: peerdb.peerdb.io/v1alpha1
kind: PeerDBCluster
metadata:
  name: peerdb
spec:
  init:
    temporalNamespaceRegistration:
      enabled: false
    temporalSearchAttributes:
      enabled: false
  # ... rest of spec
```

### Adjust Backoff Limit

If jobs fail due to transient issues, increase the retry count:

```yaml
spec:
  init:
    temporalNamespaceRegistration:
      backoffLimit: 10  # default is 4
    temporalSearchAttributes:
      backoffLimit: 10
```

### Override Job Image

If the default `temporalio/admin-tools` image has issues, you can override it:

```yaml
spec:
  init:
    temporalNamespaceRegistration:
      image: "temporalio/admin-tools:1.24.0"
    temporalSearchAttributes:
      image: "temporalio/admin-tools:1.24.0"
```
