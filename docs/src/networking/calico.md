# Using Calico

This section lists the security rules that must be implemented in the network security groups (NSGs) in order to use [Calico][calico] as a CNI provider.

## Control plane endpoint NSG

The control plane endpoint NSG will be attached to the OCI load balancer. The egress and ingress rules are listed below.

### Control plane endpoint NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                                                              |
| ---------------- | ----------------- | ----------------- | --------- | ------------------------------------------------------------------------ |
| CIDR block       | 10.0.0.0/29       |  6443             | TCP       | Allow HTTPS traffic to Control plane for Kubernetes API server access    |

### Control plane endpoint NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                       |
| ---------------- | ----------------- | ----------------- | ---------            | ------------------------------------------------  |
| CIDR block       | 0.0.0.0/0         | 6443              |  TCP                 | Allow public access to endpoint OCI load balancer     |

## Control plane NSG

The OCI compute instances running the Kubernetes control plane components will be attached to this NSG.

### Control plane NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                                     |
| ---------------- | ----------------- | ----------------- | --------- | ----------------------------------------------- |
| CIDR block       |  0.0.0.0/0        |  All              | ALL       | Control plane access to Internet to pull images |

#### Ingress Rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                                           |
| ---------------- | ----------------- | ----------------- | -------------------- | --------------------------------------------------------------------- |
| CIDR block       | 10.0.0.8/29       | 6443              |  TCP                 | Kubernetes API endpoint to Kubernetes control plane communication     |
| CIDR block       | 10.0.0.0/29       | 6443              |  TCP                 | Control plane to control plane (API server port) communication        |
| CIDR block       | 10.0.64.0/20      | 6443              |  TCP                 | Worker Node to Kubernetes control plane (API Server) communication    |
| CIDR block       | 10.0.0.0/29       | 2379              |  TCP                 | etcd client communication                                             |
| CIDR block       | 10.0.0.0/29       | 2380              |  TCP                 | etcd peer communication                                               |
| CIDR block       | 10.0.0.0/29       | 179               |  TCP                 | Calico networking (BGP)                                               |
| CIDR block       | 10.0.64.0/20      | 179               |  TCP                 | Calico networking (BGP)                                               |
| CIDR block       | 10.0.0.0/29       |                   |  IP-in-IP            | Calico networking with IP-in-IP enabled                               |
| CIDR block       | 10.0.64.0/20      |                   |  IP-in-IP            | Calico networking with IP-in-IP enabled                               |
| CIDR block       | 10.0.0.0/16       |                   |  ICMP Type 3, Code 4 | MTU Path discovery                                                    |
| CIDR block       | 0.0.0.0/0         | 22                |  TCP                 | Inbound SSH traffic to control plane nodes                            |


## Worker NSG

The OCI compute instances which running as Kubernetes worker nodes will be attached to this NSG.

### Worker NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                           |
| ---------------- | ----------------- | ----------------- | --------- | ------------------------------------- |
| CIDR block       |  0.0.0.0/0        |  All              | All       | Worker node access to Internet to pull images       |

### Worker NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                                           |
| ---------------- | ----------------- | ----------------- | -------------------- | --------------------------------------------------------------------- |
| CIDR block       | 10.0.0.32/27      | 32000-32767       |  TCP                 | Allow incoming traffic from service load balancers (NodePort Communication)                  |
| CIDR block       | 10.0.0.0/29       | 10250             |  TCP                 | Control plane to worker node (Kubelet Communication)                  |
| CIDR block       | 10.0.64.0/20      | 10250             |  TCP                 | Worker nodes to worker node (Kubelet Communication)                   |
| CIDR block       | 10.0.0.0/29       | 179               |  TCP                 | Calico networking (BGP)                                               |
| CIDR block       | 10.0.64.0/20      | 179               |  TCP                 | Calico networking (BGP)                                               |
| CIDR block       | 10.0.0.0/29       |                   |  IP-in-IP            | Calico networking with IP-in-IP enabled                               |
| CIDR block       | 10.0.64.0/20      |                   |  IP-in-IP            | Calico networking with IP-in-IP enabled                               |
| CIDR block       | 10.0.0.0/16       |                   |  ICMP Type 3, Code 4 | MTU Path discovery                                                    |
| CIDR block       | 0.0.0.0/0         | 22                |  TCP                 | Inbound SSH traffic to worker nodes                                   |

## Service Load Balancers NSG

OCI load balancers created as part of Kubernetes services of type LoadBalancer will be attached to this NSG.

### Service Load Balancers NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                           |
| ---------------- | ----------------- | ----------------- | --------- | ------------------------------------- |
| CIDR block       |  10.0.64.0/20     |  32000-32767      | TCP       | Allow access to NodePort services from Service Load balancers       |

### Service Load Balancers NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                       |
| ---------------- | ----------------- | ----------------- | ---------            | ------------------------------------------------  |
| CIDR block       | 0.0.0.0/0         |   80, 443         |  TCP                 | Allow incoming traffic to services                |

[calico]: https://www.tigera.io/project-calico/
