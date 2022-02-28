# Default Network Infrastructure

The diagram below depicts the networking architecture for a public workload cluster created in a region such as **US West (Phoenix)**.
![Networking Architecture - Workload Cluster](../images/clusteroci.svg)

Each workload cluster requires an [OCI Virtual Cloud Network (VCN)](../reference/glossary.md#vcn) which houses all the resources created for the workload cluster. The default VCN has the following resources:

* Gateways:
  1. An [Internet Gateway](../reference/glossary.md#internet-gateway)
  2. A [NAT Gateway](../reference/glossary.md#nat-gateway)
  3. A [Service Gateway](../reference/glossary.md#service-gateway)

* Route Tables:
  1. A route table for [public subnets][public-vs-private-subnets] which will route stateful traffic to and from the Internet Gateway
  2. A route table for [private subnets][public-vs-private-subnets] which will route stateful traffic to and from the NAT and Service Gateways

* Subnets:
  1. A public Control plane endpoint subnet which houses an OCI Load Balancer. The load balancer acts as a reverse proxy for the Kubernetes API Server.
  2. A private Control plane subnet which houses the Control plane nodes. The Control plane nodes run the Kubernetes Control plane components such as the API Server and the Control plane pods.
  3. A public subnet which houses the service load balancers.
  4. A private subnet which houses the worker nodes.

* Network Security Groups (NSG):
  1. An NSG for the Control plane endpoint (Control plane Endpoint NSG)
  2. An NSG for the Kubernetes Control plane nodes (Control plane NSG)
  3. An NSG for the service load balancers (Worker NSG)
  4. An NSG for the Kubernetes worker nodes (Service Load Balancers NSG)

The sections below list the security rules required for the NSGs in each of the following [CNI](../reference/glossary.md#cni) providers:

* [Using Calico](calico.md)
* [Using Antrea](antrea.md)

Currently, the following providers have been tested and verified to work:

| CNI               | CNI Version   | Kubernetes Version | CAPOCI Version |
| ----------------- | --------------| ------------------ | -------------- |
| [Calico][calico]  |     3.21      |     1.20.10        |   0.1          |
| [Antrea][antrea]  |               |     1.20.10        |   0.1          |

If you have tested an alternative CNI provider and verified it to work, please send us a PR to add it to the list. Your PR for your tested CNI provider should include the following:

* CNI provider version tested
* Documentation of NSG rules required
* A YAML template for your tested provider. See the [Antrea template](../../../templates/cluster-template-antrea.yaml) as an example.

[antrea]: antrea.md
[calico]: calico.md
[public-vs-private-subnets]: https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/overview.htm#Public
