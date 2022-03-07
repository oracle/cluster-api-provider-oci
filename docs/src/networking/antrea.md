# Using Antrea

This section lists the security rules that must be implemented in the Network Security Groups (NSGs) in order to use [Antrea][antrea] as a CNI provider.

## Control plane endpoint NSG

The Control plane Endpoint NSG will be attached to the OCI Load Balancer. The egress and ingress rules are listed below.

### Control plane endpoint NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                                                              |
| ---------------- | ----------------- | ----------------- | --------- | ------------------------------------------------------------------------ |
| CIDR Block       | 10.0.0.0/29       |  6443             | TCP       | Allow HTTPS Traffic to Control plane for Kubernetes API Server access    |

### Control plane endpoint NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                       |
| ---------------- | ----------------- | ----------------- | ---------            | ------------------------------------------------  |
| CIDR Block       | 0.0.0.0/0         | 6443              |  TCP                 | Allow public access to endpoint OCI Load Balancer     |

## Control plane NSG

The OCI Compute instances running the Kubernetes Control plane components will be attached to this NSG.

### Control plane NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                                     |
| ---------------- | ----------------- | ----------------- | --------- | ----------------------------------------------- |
| CIDR Block       |  0.0.0.0/0        |  All              | ALL       | Control plane access to Internet to pull images |

### Control plane NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                                           |
| ---------------- | ----------------- | ----------------- | -------------------- | --------------------------------------------------------------------- |
| CIDR Block       | 10.0.0.8/29       | 6443              |  TCP                 | Kubernetes API endpoint to Kubernetes Control plane communication     |
| CIDR Block       | 10.0.0.0/29       | 6443              |  TCP                 | Control plane to Control plane (API Server port) communication        |
| CIDR Block       | 10.0.64.0/20      | 6443              |  TCP                 | Worker Node to Kubernetes Control plane (API Server port)communication|
| CIDR block       | 10.0.0.0/29       | 10250             |  TCP                 | Control Plane to Control Plane Kubelet Communication                  |
| CIDR Block       | 10.0.0.0/29       | 2379              |  TCP                 | etcd client communication                                             |
| CIDR Block       | 10.0.0.0/29       | 2380              |  TCP                 | etcd peer communication                                               |
| CIDR Block       | 10.0.0.0/29       | 10349             |  TCP                 | Antrea Service                                                        |
| CIDR Block       | 10.0.64.0/20      | 10349             |  TCP                 | Antrea Service                                                        |
| CIDR Block       | 10.0.0.0/29       |  6081             |  UDP                 | Geneve Service                                                        |
| CIDR Block       | 10.0.64.0/20      |  6081             |  UDP                 | Geneve Service                                                        |
| CIDR Block       | 10.0.0.0/16       |                   |  ICMP Type 3, Code 4 | Path discovery                                                        |
| CIDR Block       | 0.0.0.0/0         | 22                |  TCP                 | Inbound SSH traffic to Control plane nodes                            |

## Worker NSG

The OCI Compute instances which running as Kubernetes worker nodes will be attached to this NSG.

### Worker NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                           |
| ---------------- | ----------------- | ----------------- | --------- | ------------------------------------- |
| CIDR Block       |  0.0.0.0/0        |  All              | All       | Worker Nodes access to Internet to pull images       |

### Worker NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                                           |
| ---------------- | ----------------- | ----------------- | -------------------- | --------------------------------------------------------------------- |
| CIDR Block       | 10.0.0.32/27      | 32000-32767       |  TCP                 | Allow incoming traffic from service load balancers (NodePort Communication)                  |
| CIDR Block       | 10.0.0.0/29       | 10250             |  TCP                 | Control plane to worker node (Kubelet Communication)                  |
| CIDR Block       | 10.0.64.0/20      | 10250             |  TCP                 | Worker nodes to worker node (Kubelet Communication)                   |
| CIDR Block       | 10.0.0.0/29       | 10349             |  TCP                 | Antrea Service                                                        |
| CIDR Block       | 10.0.64.0/20      | 10349             |  TCP                 | Antrea Service                                                        |
| CIDR Block       | 10.0.0.0/29       |  6081             |  UDP                 | Geneve Service                                                        |
| CIDR Block       | 10.0.64.0/20      |  6081             |  UDP                 | Geneve Service                                                        |
| CIDR Block       | 10.0.0.0/16       |                   |  ICMP Type 3, Code 4 | Path discovery                                                        |
| CIDR Block       | 0.0.0.0/0         | 22                |  TCP                 | Inbound SSH traffic to worker nodes                                   |

## Service load balancers NSG

OCI load balancers created as part of Kubernetes Services of type LoadBalancer will be attached to this NSG.

### Service load balancers NSG egress rules

| Destination Type | Destination       |  Destination Port | Protocol  | Description                           |
| ---------------- | ----------------- | ----------------- | --------- | ------------------------------------- |
| CIDR Block       |  10.0.64.0/20     |  32000-32767      | TCP       | Allow access to NodePort services from Service Load balancers       |

### Service load balancers NSG ingress rules

| Source Type      | Source            |  Destination Port | Protocol             | Description                                       |
| ---------------- | ----------------- | ----------------- | ---------            | ------------------------------------------------  |
| CIDR Block       | 0.0.0.0/0         |   80, 443         |  TCP                 | Allow incoming traffic to services                |

[antrea]: https://antrea.io/docs/
