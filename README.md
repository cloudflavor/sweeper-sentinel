# sweeper-sentinel

A Kubernetes operator that automatically prunes resources based on Time-to-Live (TTL) policies.

## Description

Sweeper Sentinel watches user-defined Kubernetes resources and automatically deletes them when they exceed their configured TTL. This is useful for:

- Cleaning up ephemeral test resources
- Managing temporary development environments
- Enforcing retention policies on namespaced resources
- Reducing cluster resource bloat

The operator uses dynamic watchers to monitor arbitrary resource types (GVKs) specified in SweeperSentinel custom resources, enabling flexible cleanup policies without code changes.

## Features

- **Dynamic Resource Watching**: Configure any Kubernetes resource type (Pods, Deployments, CRDs, etc.)
- **Flexible TTL Configuration**: Define retention periods using intuitive formats (e.g., "10m", "2h", "7d")
- **Multi-Resource Support**: A single SweeperSentinel can watch multiple resource types
- **Namespace Scoped**: Targets resources within specific namespaces
- **Reference Counting**: Efficiently manages watchers when multiple SweeperSentinels target the same resource type

## Architecture

The controller:
1. Watches SweeperSentinel custom resources
2. Dynamically adds watchers for each target GVK specified in the spec
3. Uses `unstructured.Unstructured` for dynamic resource handling
4. Reconciles when target resources change, checking age against TTL
5. Deletes resources that exceed their configured TTL

## Getting Started

### Prerequisites
- Go version v1.24.0+
- Docker version 17.03+
- kubectl version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

### Installation

**Install the CRDs:**

```sh
make install
```

**Deploy the operator:**

```sh
make deploy IMG=<your-registry>/sweeper-sentinel:tag
```

### Usage

Create a SweeperSentinel resource to define which GVKs should be pruned. TTLs are provided on the target resources themselves via the `sweepers.sentinel.cloudflavor.io/ttl` annotation, and resources must opt in with the `sweepers.sentinel.cloudflavor.io/enabled: "true"` label:

```yaml
apiVersion: sweepers.sentinel.cloudflavor.io/v1
kind: SweeperSentinel
metadata:
  name: dev-cleanup
  namespace: development
spec:
  targets:
    - apiVersion: "v1"
      kind: "Pod"
    - apiVersion: "apps/v1"
      kind: "Deployment"
    - apiVersion: "v1"
      kind: "ConfigMap"
```

Annotate any resource you want Sweeper Sentinel to prune:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example
  namespace: development
  labels:
    sweepers.sentinel.cloudflavor.io/enabled: "true"
  annotations:
    sweepers.sentinel.cloudflavor.io/ttl: "2h"
```

**TTL Annotation Format:**
- `s` - seconds (e.g., "30s")
- `m` - minutes (e.g., "10m")
- `h` - hours (e.g., "2h")
- `d` - days (e.g., "7d")

### Examples

> **RBAC NOTE:** The controller requires wildcard list/watch/delete access so it can manage whatever GVKs you specify. Scope Sweeper Sentinel to dedicated namespaces and only label/annotate resources you explicitly want pruned.

**Example 1: Clean up test Pods after 10 minutes**
```yaml
apiVersion: sweepers.sentinel.cloudflavor.io/v1
kind: SweeperSentinel
metadata:
  name: pod-cleanup
spec:
  targets:
    - apiVersion: "v1"
      kind: "Pod"
```

**Example 2: Comprehensive ephemeral resource cleanup**
```yaml
apiVersion: sweepers.sentinel.cloudflavor.io/v1
kind: SweeperSentinel
metadata:
  name: ephemeral-cleanup
spec:
  targets:
    - apiVersion: "v1"
      kind: "Pod"
    - apiVersion: "apps/v1"
      kind: "Deployment"
    - apiVersion: "apps/v1"
      kind: "StatefulSet"
    - apiVersion: "batch/v1"
      kind: "Job"
    - apiVersion: "v1"
      kind: "Service"
    - apiVersion: "v1"
      kind: "ConfigMap"
```

**Example 3: Watch custom resources**
```yaml
apiVersion: sweepers.sentinel.cloudflavor.io/v1
kind: SweeperSentinel
metadata:
  name: custom-resource-cleanup
spec:
  targets:
    - apiVersion: "myapp.example.com/v1"
      kind: "MyCustomResource"
```

## Development

### Running locally

```sh
make run
```

### Running tests

```sh
make test
```

### Running e2e tests

```sh
make test-e2e
```

### Linting

```sh
make lint
```

## How It Works

1. **Controller Initialization**: On startup, the controller lists existing SweeperSentinel resources and sets up dynamic watchers for all target GVKs
2. **Dynamic Watching**: Uses `unstructured.Unstructured` to watch arbitrary resource types without compile-time type knowledge
3. **Reference Counting**: Tracks how many SweeperSentinels watch each GVK to avoid duplicate watchers
4. **Reconciliation**: When target resources change or SweeperSentinels are modified, checks resource age against TTL
5. **Deletion**: Resources exceeding their TTL are deleted automatically

## RBAC Considerations

The operator requires permissions to:
- Watch and list SweeperSentinel resources
- Watch, list, and delete target resources specified in SweeperSentinel specs

Ensure your RBAC configuration grants appropriate permissions for the resources you want to manage.

## To Uninstall

**Delete SweeperSentinel instances:**

```sh
kubectl delete sweepersentinels --all
```

**Delete the CRDs:**

```sh
make uninstall
```

**Undeploy the controller:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/sweeper-sentinel:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/sweeper-sentinel/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
   can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

Contributions are welcome! Please ensure:
- Code passes `make lint` and `make test`
- Changes include appropriate test coverage
- Documentation is updated for new features

**NOTE:** Run `make help` for more information on all potential `make` targets

## License

Copyright 2025 Cloudflavor.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## More Information

For more information on Kubebuilder and controller patterns:
- [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
- [Controller Runtime Documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)