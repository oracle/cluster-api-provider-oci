# Networking architecture for a workload cluster

Each workload cluster requires an Oracle Cloud Infrastructure (OCI) Virtual Cloud Network (VCN) which will house all the resources created for the workload cluster. The default VCN will have the following resources:

* Gateways:
  1. An Internet gateway.
  2. A NAT gateway.
  3. A service gateway.

* Route Tables:
  1. A route table for public subnets which will route stateful traffic to and from the Internet Gateway.
  2. A route table for private subnets which will route stateful traffic to and from the NAT and Service Gateways.

* Subnets:
  1. A public control plane endpoint subnet which houses an OCI Load Balancer. The load balancer acts as a reverse proxy for the Kubernetes API Server.
  2. A private control plane subnet which houses the control plane nodes. The control plane nodes run the Kubernetes control plane components such as the API server and the control plane pods.
  3. A public subnet which houses the service load balancers.
  4. A private subnet which houses the worker nodes.

* Network Security Groups (NSG):
  1. An NSG for the control plane endpoint
  2. An NSG for the Kubernetes control plane nodes
  3. An NSG for the service load balancers
  4. An NSG for the Kubernetes worker nodes

The default security rules in the NSGs listed below assumes Calico as CNI provider. The rules will need to be reviewed and possibly changed to support other CNI providers.

## Network security groups (NSG)

The following types of network security groups are to be configured.

### Control plane endpoint NSG

The Control Plane Endpoint NSG will be attached to the Load Balancer. The egress and ingress rules are listed below.

#### Control plane endpoint NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                           |
| ---------------- | ----------------- | ----------------- | --------- | ------------------------------------- |
| CIDR block       | 10.0.0.0/29       |  6443             | TCP       | Allow HTTPS traffic to API Server     |
| CIDR block       | 10.0.0.0/29       |                   | ICMP      | Allow MTU path discovery to control plane |

#### Control plane endpoint NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                       |
| ---------------- | ----------------- | ----------------- | ---------            | ------------------------------------------------  |
| CIDR block       | 0.0.0.0/0         | 6443              |  TCP                 | Allow public access to Kubernetes API endpoint    |
| CIDR block       | 10.0.0.0/29       | 6443              |  TCP                 | Allow Control Plane nodes to API Server endpoint  |
| CIDR block       | 0.0.0.0/0         |                   |  ICMP Type 3, Code 4 | Allow MTU path discovery from endpoint load balancers |

### Control plane NSG

The OCI Compute instances running the Kubernetes ControlPlane components will be attached to this NSG.

#### Control plane NSG ingress rules

| Description                                                            | Protocol | Destination Ports | Source Type | CIDR Range   |
|------------------------------------------------------------------------|----------|-------------------|-------------|--------------|
| Kubernetes API endpoint to Kubernetes control plane communication      | TCP      | 6443              | CIDR Block  | 10.0.0.8/29  |
| Control plane to control plane (apiserver port) communication          | TCP      | 6443              | CIDR Block  | 10.0.64.0/20 |
| Worker node to Kubernetes control plane (apiserver port) communication | TCP      | 6443              | CIDR Block  | 10.0.0.0/29  |
| etcd client communication                                              | TCP      | 2379              | CIDR Block  | 10.0.0.0/29  |
| etcd peer communication                                                | TCP      | 2380              | CIDR Block  | 10.0.0.0/29  |
| Calico networking (BGP)                                                | TCP      | 179               | CIDR Block  | 10.0.0.0/29  |
| Calico networking (BGP)                                                | TCP      | 179               | CIDR Block  | 10.0.64.0/20 |
| Calico networking with IP-in-IP enabled                                | IP-in-IP | NA                | CIDR Block  | 10.0.0.0/29  |
| Calico networking with IP-in-IP enabled                                | IP-in-IP | NA                | CIDR Block  | 10.0.64.0/20 |
| MTU Path discovery                                                     | ICMP     | ICMP 3,4          | CIDR Block  | 10.0.0.0/16  |
| Inbound SSH traffic to control plane nodes                             | TCP      | 22                | CIDR Block  | 0.0.0.0/0    |

#### Control plane NSG egress rules

| Description                      | Protocol | Ports | Destination Type | CIDR Range |
|----------------------------------|----------|-------|------------------|------------|
| Control Plane access to Internet | All      | All   | CIDR Block       | 0.0.0.0/0  |

### Worker node NSG

The OCI compute instances which running as Kubernetes worker nodes will be attached to this NSG.

#### Worker node NSG ingress rules

| Description                                                                         | Protocol | Destination Ports | Source Type | CIDR Range   |
|-------------------------------------------------------------------------------------|----------|-------------------|-------------|--------------|
| Control plane to worker node Kubelet communication                                  | TCP      | 10250             | CIDR Block  | 10.0.0.0/29  |
| Worker nodes to worker node Kubelet communication                                   | TCP      | 10250             | CIDR Block  | 10.0.64.0/20 |
| Allow pods on one worker node to communicate with pods on other control plane nodes | All      | All               | CIDR Block  | 10.0.0.0/29  |
| Calico networking (BGP)                                                             | TCP      | 179               | CIDR Block  | 10.0.0.0/29  |
| Calico networking (BGP)                                                             | TCP      | 179               | CIDR Block  | 10.0.64.0/20 |
| Calico networking with IP-in-IP enabled                                             | IP-in-IP | NA                | CIDR Block  | 10.0.0.0/29  |
| Calico networking with IP-in-IP enabled                                             | IP-in-IP | NA                | CIDR Block  | 10.0.64.0/20 |
| MTU Path discovery                                                                  | ICMP     | ICMP 3,4          | CIDR Block  | 10.0.0.0/16  |
| Inbound SSH traffic to worker nodes                                                 | TCP      | 22                | CIDR Block  | 0.0.0.0/0    |

##### Egress Rules

| Description                     | Protocol | Ports | Destination Type | CIDR Range |
|---------------------------------|----------|-------|------------------|------------|
| Worker nodes access to Internet | All      | All   | CIDR Block       | 0.0.0.0/0  |

#### Service Load Balancers NSG

OCI Load balancers created as part of Kubernetes Services of type `LoadBalancer` will be attached to this NSG.

##### Ingress Rules

| Description    | Protocol | Source Ports | Source Type | CIDR Range  |
|----------------|----------|--------------|-------------|-------------|
| MTU Path discovery | ICMP     | ICMP 3,4     | CIDR Block  | 10.0.0.0/16 |

### Network Diagram

The diagram below depicts the networking architecture for a workload cluster created in an OCI multi-AD region such as us-phoenix-1.
![Networking Architecture - Workload Cluster](../images/multiad_public_cluster.svg)
