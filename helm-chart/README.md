# Indonesian Regions Fuzzy Search API - Helm Chart

This Helm chart deploys the Indonesian Regions Fuzzy Search API to a Kubernetes cluster. The API provides fast and accurate fuzzy search capabilities for Indonesian administrative regions (provinces, cities, districts, and subdistricts) using DuckDB.

## Table of Contents

- [Description](#description)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
  - [Quick Start](#quick-start)
  - [Custom Installation](#custom-installation)
- [Configuration](#configuration)
  - [Global Configuration](#global-configuration)
  - [Image Configuration](#image-configuration)
  - [Service Configuration](#service-configuration)
  - [Ingress Configuration](#ingress-configuration)
  - [Persistence Configuration](#persistence-configuration)
  - [Resource Configuration](#resource-configuration)
  - [Autoscaling Configuration](#autoscaling-configuration)
  - [Network Policy Configuration](#network-policy-configuration)
  - [Service Account Configuration](#service-account-configuration)
  - [Environment Variables](#environment-variables)
- [Example Usage Scenarios](#example-usage-scenarios)
  - [Basic Deployment](#basic-deployment)
  - [Deployment with Ingress](#deployment-with-ingress)
  - [Deployment with Persistence](#deployment-with-persistence)
  - [High Availability Deployment](#high-availability-deployment)
- [Accessing the Service](#accessing-the-service)
- [Troubleshooting](#troubleshooting)
  - [Common Issues](#common-issues)
  - [Debugging Commands](#debugging-commands)
- [Contributing](#contributing)
- [Architecture](#architecture)

## Description

The Indonesian Regions Fuzzy Search API is a high-performance, dependency-free Go API that provides fuzzy search capabilities for Indonesian administrative regions. This Helm chart simplifies deployment on Kubernetes clusters with configurable options for various environments.

Key features of the API:
- **Fuzzy Search**: Uses Levenshtein distance algorithm for typo-tolerant searches
- **High Performance**: Powered by DuckDB for fast querying of Indonesian administrative data
- **Lightweight**: Minimal dependencies with GoFiber web framework
- **Container Ready**: Dockerized application for easy deployment
- **Configurable**: Environment-based configuration for port and database path

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Persistent storage support in the cluster (for persistence feature)
- Ingress controller (for ingress feature)

## Installation

### Quick Start

To install the chart with the release name `my-release`:

```bash
helm install my-release ./helm-chart
```

This will deploy the API with default settings, which includes:
- Single replica
- ClusterIP service
- Persistent storage enabled
- Basic resource limits

### Custom Installation

To customize the installation, you can override values using a custom values file:

```bash
helm install my-release ./helm-chart -f values.yaml
```

Or override specific values directly:

```bash
helm install my-release ./helm-chart \
  --set replicaCount=3 \
  --set service.type=LoadBalancer \
  --set ingress.enabled=true
```

## Configuration

The following table lists the configurable parameters of the chart and their default values.

### Global Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Image repository | `wilayah-indonesia` |
| `image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override full name | `""` |

### Image Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `image.repository` | Container image repository | `wilayah-indonesia` |
| `image.tag` | Container image tag | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Service Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.targetPort` | Target port on the container | `8080` |
| `service.annotations` | Service annotations | `{}` |

### Ingress Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `ingress.enabled` | Enable ingress controller resource | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress hosts configuration | `[{host: chart-example.local, paths: [{path: /, pathType: ImplementationSpecific}]}]` |
| `ingress.tls` | TLS configuration for ingress | `[]` |

### Persistence Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `persistence.enabled` | Enable persistence | `true` |
| `persistence.storageClass` | Storage class name | `""` |
| `persistence.accessModes` | Access modes for the persistent volume | `["ReadWriteOnce"]` |
| `persistence.size` | Size of the persistent volume | `1Gi` |
| `persistence.existingClaim` | Use an existing PVC | `""` |

### Resource Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `resources.limits.cpu` | CPU limit for the container | `100m` |
| `resources.limits.memory` | Memory limit for the container | `128Mi` |
| `resources.requests.cpu` | CPU request for the container | `100m` |
| `resources.requests.memory` | Memory request for the container | `128Mi` |

### Autoscaling Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `autoscaling.enabled` | Enable horizontal pod autoscaling | `false` |
| `autoscaling.minReplicas` | Minimum number of replicas | `1` |
| `autoscaling.maxReplicas` | Maximum number of replicas | `100` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU utilization percentage | `80` |

### Network Policy Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `networkPolicy.enabled` | Enable network policy | `false` |
| `networkPolicy.annotations` | Network policy annotations | `{}` |
| `networkPolicy.ingress.enabled` | Enable ingress rules | `true` |
| `networkPolicy.ingress.cidr` | CIDR block to allow ingress from | `""` |
| `networkPolicy.egress.enabled` | Enable egress rules | `true` |

### Service Account Configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `serviceAccount.create` | Specifies whether a service account should be created | `true` |
| `serviceAccount.annotations` | Annotations to add to the service account | `{}` |
| `serviceAccount.name` | The name of the service account to use | `""` |

### Environment Variables

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `env.PORT` | Port on which the application listens | `"8080"` |
| `env.DB_PATH` | Path to the database file | `"/data/regions.duckdb"` |

For more details on configuring the chart, refer to the [values.yaml](values.yaml) file.

## Example Usage Scenarios

### Basic Deployment

Deploy a single instance of the API with default settings:

```bash
helm install regions-api ./helm-chart
```

### Deployment with Ingress

Deploy the API with an ingress controller for external access:

```bash
helm install regions-api ./helm-chart \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=regions-api.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix
```

### Deployment with Persistence

Deploy the API with custom persistence settings:

```bash
helm install regions-api ./helm-chart \
  --set persistence.size=5Gi \
  --set persistence.storageClass=fast-ssd
```

### High Availability Deployment

Deploy the API with multiple replicas and autoscaling:

```bash
helm install regions-api ./helm-chart \
  --set replicaCount=3 \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=3 \
  --set autoscaling.maxReplicas=10
```

## Accessing the Service

After deployment, you can access the service in several ways depending on your configuration:

### ClusterIP (Default)

Access the service internally within the cluster:

```bash
kubectl port-forward service/<release-name>-wilayah-indonesia 8080:8080
```

Then access the API at `http://localhost:8080`

### NodePort

If you've configured the service as NodePort:

```bash
kubectl get nodes -o wide
```

Find the external IP of any node and access the service at `http://<node-ip>:<node-port>`

### LoadBalancer

If you've configured the service as LoadBalancer:

```bash
kubectl get services
```

Find the external IP of the load balancer and access the service at `http://<external-ip>:8080`

### Ingress

If you've enabled ingress, access the service through your configured domain:

```bash
curl http://regions-api.example.com/v1/search?q=bandung
```

## Troubleshooting

### Common Issues

1. **Service not accessible**: Check if the service is running and the port configurations are correct:
   ```bash
   kubectl get pods
   kubectl describe service <release-name>-wilayah-indonesia
   ```

2. **Database not found**: If persistence is enabled, ensure the PVC is properly bound:
   ```bash
   kubectl get pvc
   kubectl describe pvc <release-name>-wilayah-indonesia-data
   ```

3. **Ingress not working**: Verify the ingress controller is installed and the ingress resource is created:
   ```bash
   kubectl get ingress
   kubectl describe ingress <release-name>-wilayah-indonesia
   ```

4. **Insufficient resources**: If pods are stuck in pending state, check resource quotas:
   ```bash
   kubectl describe pods
   ```

### Debugging Commands

Check the status of all resources created by the chart:

```bash
kubectl get all -l app.kubernetes.io/name=wilayah-indonesia
```

View logs of the API pod:

```bash
kubectl logs -l app.kubernetes.io/name=wilayah-indonesia
```

Check events for troubleshooting:

```bash
kubectl get events --sort-by=.metadata.creationTimestamp
```

Exec into the pod for debugging:

```bash
kubectl exec -it <pod-name> -- /bin/sh
```

## Contributing

We welcome contributions to improve this Helm chart! Here's how you can contribute:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test your changes thoroughly
5. Submit a pull request

Please ensure your changes:
- Follow Helm best practices
- Include updated documentation
- Pass linting checks (`helm lint`)
- Are tested with `helm template`

## Architecture

This chart deploys the following components:

- Deployment for the API service
- Service to expose the API internally
- Optional Ingress for external access
- PersistentVolumeClaim for data storage
- Optional HorizontalPodAutoscaler for scaling
- Optional NetworkPolicy for network security
- ServiceAccount with associated Role and RoleBinding for RBAC

The application uses DuckDB for data storage, which is persisted using a PersistentVolumeClaim. The API exposes endpoints for searching Indonesian administrative regions with fuzzy matching capabilities.