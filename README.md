# Kubernetes Cluster API Provider OCI

[![Go Report Card](https://goreportcard.com/badge/github.com/oracle/cluster-api-provider-oci)](https://goreportcard.com/report/github.com/oracle/cluster-api-provider-oci)

<!-- markdownlint-disable MD033 -->
<img src="https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png"  width="100">

------
Kubernetes-native declarative infrastructure for OCI.

## Kubernetes Cluster API Provider for Oracle Cloud Infrastructure

The [Cluster API Provider for OCI (CAPOCI)][cluster_api] brings declarative, Kubernetes-style APIs to cluster creation, configuration and management.

The Cluster API itself is shared across multiple cloud providers allowing for true hybrid deployments of Kubernetes.

### Features

- Self-managed and Container Engine for Kubernetes(OKE) clusters
- Manages the bootstrapping of VCNs, gateways, subnets, network security groups
- Provide secure and sensible defaults

### Getting Started

You can find detailed documentation as well as a getting started guide in the [Cluster API Provider for OCI Book][capoci_book].

## 🤗 Community

The CAPOCI provider is developed in the open, and is constantly being improved
by our users, contributors, and maintainers.

To ask questions or get the latest project news, please join us in the
[#cluster-api-oci][#cluster-api-oci] channel on [Slack](http://slack.k8s.io/).

### Office Hours

The maintainers host office hours on the first Tuesday of every month
at 06:00 PT / 09:00 ET / 14:00 CET / 18:00 IST via [Zooom][zoomMeeting].

All interested community members are invited to join us. A recording of each
session will be made available afterwards for folks who are unable to attend.

Previous meetings: [ [notes][notes] | [recordings][notes] (coming soon) ]

### Support Policy

**NOTE:** As the versioning for this project is tied to the versioning of Cluster API, future modifications to this
policy may be made to more closely align with other providers in the Cluster API ecosystem.

### Cluster API Versions

CAPOCI supports the following Cluster API versions.

|                          | Cluster API `v1beta1` (`v1.x.x`) |
|--------------------------|----------------------------------|
| OCI Provider `(v0.x.x)`  | ✓                                |


### Kubernetes versions

CAPOCI provider is able to install and manage the [versions of Kubernetes supported by
Cluster API (CAPI)](https://cluster-api.sigs.k8s.io/reference/versions.html#supported-kubernetes-versions).

## Contributing

This project welcomes contributions from the community. Before submitting a pull request, please
[review our contribution guide](./CONTRIBUTING.md)

## Security

Please consult the [security guide](./SECURITY.md) for our responsible security vulnerability disclosure process

## License

Copyright (c) [2021, 2022] year Oracle and/or its affiliates.

Released under the Apache License License Version 2.0 as shown at http://www.apache.org/licenses/.

[cluster_api]: https://github.com/oracle/cluster-api-provider-oci
[image_builder_book]: https://image-builder.sigs.k8s.io/capi/providers/oci.html
[capoci_book]: https://oracle.github.io/cluster-api-provider-oci/
[#cluster-api-oci slack]: https://kubernetes.slack.com/archives/C039CDHABFF
[zoomMeeting]: https://oracle.zoom.us/j/97952312891?pwd=NlFnMWQzbGpMRmNyaityVHNWQWxSdz09
[notes]: https://docs.google.com/document/d/1mgZxjDbnSv74Vut1aHtWplG6vsR9zu5sqXvQN8SPgCc
