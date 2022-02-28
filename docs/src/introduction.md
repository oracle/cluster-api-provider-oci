# Kubernetes Cluster API Provider for Oracle Cloud Infrastructure

[![Go Report Card](https://goreportcard.com/badge/oracle/cluster-api-provider-oci)](https://goreportcard.com/report/oracle/cluster-api-provider-oci)

<!-- markdownlint-disable MD033 -->
<img src="https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png"  width="100">

------
Kubernetes-native declarative infrastructure for Oracle Cloud Infrastructure (OCI).

## What is the Cluster API Provider for OCI

The [Cluster API Provier for OCI (CAPOCI)][cluster_api] brings declarative, Kubernetes-style APIs to cluster creation, configuration and management.

The API itself is shared across multiple cloud providers allowing for true OCI
hybrid deployments of Kubernetes.

## Features

- Manages the bootstrapping of VCNs, gateways, subnets, network security groups and instances
- Choice of Linux distribution between Oracle Linux and Ubuntu
- Deploys Kubernetes control plane into private subnets front-ended by a public load balancer.
- Provides secure and sensible defaults.

## Getting Started

- [Prerequisites][prerequisites]: Set up your OCI tenancy before using Cluster API for OCI.
- [Deployment process][deployment]: Choose your deployment path
- [Networking][networking]: Networking guide
- Installation:
  - [Install Cluster API for OCI][install_cluster_api]
  - [Install Workload Cluster][install_workload_cluster]

## Support Policy

Cluster API and Kubernetes version compatibility

### Cluster API Versions

|                              | v1beta1 (v1.0) |
| ---------------------------- | -------------- |
| OCI Provider v1beta1 (v0.1)  |        ✓       |

### Supported Kubernetes versions

|                              | v1.20 | v1.21 |
| ---------------------------- | ----- | ----- |
| OCI Provider v1beta1 (v0.1)  |   ✓   |   ✓  |

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this
policy may be made to more closely align with other providers in the Cluster API ecosystem.

[cluster_api]: https://github.com/kubernetes-sigs/cluster-api-oci
[deployment]: ./gs/overview.md
[install_cluster_api]: ./gs/install_cluster_api.md
[install_workload_cluster]: ./gs/install_workload_cluster.md
[networking]: ./networking/networking.md
[prerequisites]: ./prerequisites.md
