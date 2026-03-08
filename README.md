# PeerDB Operator

The PeerDB Operator is a Kubernetes operator that automates the deployment, scaling, and lifecycle management of [PeerDB](https://github.com/PeerDB-io/peerdb) clusters on Kubernetes — a Postgres-first ETL/ELT platform for streaming data between databases.

It provides declarative cluster management through custom resources, enabling users to easily create and scale PeerDB deployments with independently-scalable worker pools.

## Features

- **PeerDB Cluster Management**: Create and manage PeerDB control plane (Flow API, PeerDB Server, UI)
- **Independent Worker Scaling**: CDC Flow Workers and Snapshot Workers scale independently via separate CRDs
- **HPA/KEDA Support**: Built-in autoscaling support for worker pools without operator conflicts
- **Multiple Worker Pools**: Different sizing, node selectors, and tolerations per workload profile
- **Scale-to-Zero**: Snapshot workers can scale to zero when no initial loads are running
- **Automatic Lifecycle Management**: OwnerReferences enable automatic garbage collection on CR deletion
- **Maintenance Mode Integration**: Gracefully pauses mirrors before upgrades and resumes them after via PeerDB's maintenance workflows

## Getting Started

### Prerequisites
- go version v1.26.0+
- docker version 17.03+
- `kubectl` version v1.31.0+
- Access to a Kubernetes v1.31.0+ cluster
- External PostgreSQL instance (catalog database)
- External Temporal server

### Quick Start

For users who want to quickly try the operator:

1. Install the CRDs and operator:
   - Using pre-built manifests:
     ```sh
     kubectl apply -f https://github.com/Neurostep/peerdb-operator/releases/latest/download/install.yaml
     ```
   - Using Helm chart:
     ```sh
     helm install peerdb-operator oci://ghcr.io/neurostep/peerdb-operator-helm \
        --create-namespace \
        -n peerdb-operator-system
     ```

2. Deploy a sample cluster:
   ```sh
   kubectl apply -f https://raw.githubusercontent.com/Neurostep/peerdb-operator/refs/heads/main/config/samples/peerdb_v1alpha1_peerdbcluster.yaml
   ```

3. Verify the deployment:
   ```sh
   kubectl get peerdbclusters
   kubectl get peerdbworkerpools
   kubectl get peerdbsnapshotpools
   kubectl get pods
   ```

### Deploy from Sources

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/peerdb-operator:tag
```

**NOTE:** This image ought to be pushed to the personal registry you specified.
Make sure you have the proper permission to the registry if the above commands don't work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/peerdb-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

## Examples

The `config/samples/` directory contains example CR manifests:

- **peerdb_v1alpha1_peerdbcluster.yaml**: PeerDB control plane with Flow API, Server, and UI
- **peerdb_v1alpha1_peerdbworkerpool.yaml**: CDC Flow Worker pool with autoscaling
- **peerdb_v1alpha1_peerdbsnapshotpool.yaml**: Snapshot Worker pool with persistent storage

To deploy any example:

```sh
kubectl apply -f config/samples/<example-file>.yaml
```

### To Uninstall

**Delete the CRs from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the CRDs from the cluster:**

```sh
make uninstall
```

**Undeploy the controller from the cluster:**

```sh
make undeploy
```

## Documentation

For more detailed information, see the [documentation](docs/README.md).

## Contributing

We welcome contributions to the PeerDB Operator project! Here's how you can help:

- **Bug Reports**: Open an issue describing the bug and steps to reproduce
- **Feature Requests**: Submit an issue with your feature proposal and use case
- **Pull Requests**: Fork the repository, make your changes, and submit a PR

Before contributing, please ensure:
- Run `make manifests generate` after modifying CRD types
- All code compiles: `go build ./...`
- Code passes linting: `go vet ./...`
- Commits are well-documented

## Useful Links

* [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
* [PeerDB Documentation](https://docs.peerdb.io/)
* [PeerDB GitHub](https://github.com/PeerDB-io/peerdb)

## License

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
