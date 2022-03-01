# Kubernetes Cluster API Provider OCI

[![Go Report Card](https://goreportcard.com/badge/github.com/oracle/cluster-api-provider-oci)](https://goreportcard.com/report/github.com/oracle/cluster-api-provider-oci)

<!-- markdownlint-disable MD033 -->
<img src="https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png"  width="100">

------
Kubernetes-native declarative infrastructure for OCI.

## Kubernetes Cluster API Provider for Oracle Cloud Infrastructure

The [Cluster API Provier for OCI (CAPOCI)][cluster_api] brings declarative, Kubernetes-style APIs to cluster creation, configuration and management.

The Cluster API itself is shared across multiple cloud providers allowing for true hybrid deployments of Kubernetes.

### Features

- Manages the bootstrapping of VCNs, gateways, subnets, network security groups and instances
- Deploy either Oracle Linux or Ubuntu based instances using custom images built with the [Image Builder][image_builder_book] tool
- Deploys Kubernetes Control plane into private subnets front-ended by a public load balancer
- Provide secure and sensible defaults

### Getting Started

You can find detailed documentation as well as a getting started guide in the [Cluster API Provider for OCI Book][capoci_book].

### Support Policy

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this
policy may be made to more closely align with other providers in the Cluster API ecosystem.

#### Cluster API Versions

|                                | v1beta1 (v1.0)   |
| ------------------------------ | :--------------: |
| OCI Provider v1beta1 (v0.1.0)  |       ✓          |

#### Supported Kubernetes versions

|                                | v1.20   | v1.21   |
| ------------------------------ | :-----: | :-----: |
| OCI Provider v1beta1 (v0.1.0)  |   ✓     |   ✓     |

[cluster_api]: https://github.com/oracle/cluster-api-provider-oci
[image_builder_book]: https://image-builder.sigs.k8s.io/capi/providers/oci.html
[capoci_book]: https://oracle.github.io/cluster-api-provider-oci/
