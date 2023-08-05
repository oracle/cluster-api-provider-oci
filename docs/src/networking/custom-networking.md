# Custom networking

The [default networking](infrastructure.md) can be modified to achieve the following:

- your own CIDR range for VCN. This is useful if you want to perform peering with another VCN or another cloud provider and you need to avoid IP Overlapping
- your own custom security rules using [NSGs](../reference/glossary.md#nsg). This is useful if you want to use your own CNI provider and it has a different security posture than the default
- your own custom security rules using network security lists
- change the masks and name of your different subnets. This is useful to either expand or constrain the size of your subnets as well as to use your own preferred naming convention

The `OCICluster` spec in the cluster templates can be modified to customize the network spec.

## Example spec for custom CIDR range

The spec below shows how to change the CIDR range of the VCN from the default `10.0.0.0/16` to `172.16.0.0/16`.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    vcn:
      name: ${CLUSTER_NAME}
      cidr: "172.16.0.0/16"
      subnets:
        - name: ep-subnet
          role: control-plane-endpoint
          type: public
          cidr: "172.16.0.0/28"
        - name: cp-mc-subnet
          role: control-plane
          type: private
          cidr: "172.16.5.0/28"
        - name: worker-subnet
          role: worker
          type: private
          cidr: "172.16.10.0/24"
        - name: svc-lb-subnet
          role: service-lb
          type: public
          cidr: "172.16.20.0/24"
```

## Example spec to modify default NSG security rules

The spec below shows how to change the default NSG rules.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    vcn:
      name: ${CLUSTER_NAME}
      cidr: "172.16.0.0/16"
      networkSecurityGroup:
        list:
          - name: ep-nsg
            role: control-plane-endpoint
            egressRules:
              - egressRule:
                  isStateless: false
                  destination: "172.16.5.0/28"
                  protocol: "6"
                  destinationType: "CIDR_BLOCK"
                  description: "All traffic to control plane nodes"
                  tcpOptions:
                    destinationPortRange:
                      max: 6443
                      min: 6443
            ingressRules:
              - ingressRule:
                  isStateless: false
                  source: "0.0.0.0/0"
                  protocol: "6"
                  sourceType: "CIDR_BLOCK"
                  description: "External access to Kubernetes API endpoint"
                  tcpOptions:
                    destinationPortRange:
                      max: 6443
                      min: 6443
              - ingressRule:
                  isStateless: false
                  source: "172.16.5.0/28"
                  protocol: "6"
                  sourceType: "CIDR_BLOCK"
                  description: "Control plane worker nodes to API Server endpoint"
              - ingressRule:
                  isStateless: false
                  source: "0.0.0.0/0"
                  protocol: "6"
                  sourceType: "CIDR_BLOCK"
                  description: "SSH access"
                  tcpOptions:
                    destinationPortRange:
                      max: 22
                      min: 22
          - name: cp-mc-nsg
            role: control-plane
            egressRules:
              - egressRule:
                  isStateless: false
                  destination: "0.0.0.0/0"
                  protocol: "6"
                  destinationType: "CIDR_BLOCK"
                  description: "control plane machine access to internet"
            ingressRules:
              - ingressRule:
                  isStateless: false
                  source: "172.16.0.0/16"
                  protocol: "all"
                  sourceType: "CIDR_BLOCK"
                  description: "Allow inter vcn communication"
              - ingressRule:
                  isStateless: false
                  source: "0.0.0.0/0"
                  protocol: "6"
                  sourceType: "CIDR_BLOCK"
                  description: "SSH access"
                  tcpOptions:
                    destinationPortRange:
                      max: 22
                      min: 22
          - name: worker-nsg
            role: worker
            egressRules:
              - egressRule:
                  isStateless: false
                  destination: "0.0.0.0/0"
                  protocol: "6"
                  destinationType: "CIDR_BLOCK"
                  description: "Worker Nodes access to Internet"
            ingressRules:
              - ingressRule:
                  isStateless: false
                  source: "172.16.0.0/16"
                  protocol: "all"
                  sourceType: "CIDR_BLOCK"
                  description: "Allow inter vcn communication"
          - name: service-lb-nsg
            role: service-lb
            ingressRules:
              - ingressRule:
                  isStateless: false
                  source: "172.16.0.0/16"
                  protocol: "all"
                  sourceType: "CIDR_BLOCK"
                  description: "Allow ingress from vcn subnets"
      subnets:
        - name: ep-subnet
          role: control-plane-endpoint
          type: public
          cidr: "172.16.0.0/28"
        - name: cp-mc-subnet
          role: control-plane
          type: private
          cidr: "172.16.5.0/28"
        - name: worker-subnet
          role: worker
          type: private
          cidr: "172.16.10.0/24"
        - name: svc-lb-subnet
          role: service-lb
          type: public
          cidr: "172.16.20.0/24"
```

## Example spec to use Security Lists instead of Network Security Groups

The spec below shows how to implement the security posture using security lists instead of NSGs.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    vcn:
      name: ${CLUSTER_NAME}
      subnets:
        - name: ep-subnet
          role: control-plane-endpoint
          type: public
          securityList:
            name: ep-seclist
            egressRules:
              - destination: "10.0.0.0/29"
                protocol: "6"
                destinationType: "CIDR_BLOCK"
                description: "All traffic to control plane nodes"
                tcpOptions:
                  destinationPortRange:
                    max: 6443
                    min: 6443
            ingressRules:
              - source: "0.0.0.0/0"
                protocol: "6"
                sourceType: "CIDR_BLOCK"
                description: "External access to Kubernetes API endpoint"
                tcpOptions:
                  destinationPortRange:
                    max: 6443
                    min: 6443
              - source: "10.0.0.0/29"
                protocol: "6"
                sourceType: "CIDR_BLOCK"
                description: "Control plane worker nodes to API Server endpoint"
              - source: "0.0.0.0/0"
                protocol: "6"
                sourceType: "CIDR_BLOCK"
                description: "SSH access"
                tcpOptions:
                  destinationPortRange:
                    max: 22
                    min: 22
        - name: cp-mc-subnet
          role: control-plane
          type: private
          securityList:
            name: cp-mc-seclist
            egressRules:
              - destination: "0.0.0.0/0"
                protocol: "6"
                destinationType: "CIDR_BLOCK"
                description: "control plane machine access to internet"
            ingressRules:
              - source: "10.0.0.0/16"
                protocol: "all"
                sourceType: "CIDR_BLOCK"
                description: "Allow inter vcn communication"
              - source: "0.0.0.0/0"
                protocol: "6"
                sourceType: "CIDR_BLOCK"
                description: "SSH access"
                tcpOptions:
                  destinationPortRange:
                    max: 22
                    min: 22
        - name: worker-subnet
          role: worker
          type: private
          securityList:
            name: node-seclist
            egressRules:
              - destination: "0.0.0.0/0"
                protocol: "6"
                destinationType: "CIDR_BLOCK"
                description: "Worker Nodes access to Internet"
            ingressRules:
              - source: "10.0.0.0/16"
                protocol: "all"
                sourceType: "CIDR_BLOCK"
                description: "Allow inter vcn communication"
        - name: svc-lb-subnet
          role: service-lb
          type: public
          securityList:
            name: service-lb-seclist
            ingressRules:
              - source: "10.0.0.0/16"
                protocol: "all"
                sourceType: "CIDR_BLOCK"
                description: "Allow ingress from vcn subnets"
```

Related documentation: [comparison of Security Lists and Network Security Groups][sl-vs-nsg]

## Example spec for externally managed VCN infrastructure

```admonish info
See [externally managed infrastructure][externally-managed-cluster-infrastructure] for how to create a cluster
using existing VCN infrastructure.
```

## Example spec to use OCI Load Balancer as API Server load balancer

By default, CAPOCI uses [OCI Network Load Balancer][oci-nlb] as API Server load balancer. The load balancer front-ends
control plane hosts to provide high availability access to Kubernetes API. The following spec can be used to 
use [OCI Load Balancer][oci-lb] as the API Server load balancer. The change from the default spec is to set 
`loadBalancerType` field to "lb" in the `OCICluster` resource.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    apiServerLoadBalancer:
      loadBalancerType: "lb"
```

## Example spec to use custom role

CAPOCI can be used to create Subnet/NSG in the VCN for custom workloads such as private load balancers,
dedicated subnet for DB connection etc. The roles for such custom subnest must be defined as `custom`.
The following spec shows an example for this scenario.

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    vcn:
      name: ${CLUSTER_NAME}
      subnets:
        - name: db
          role: custom
          type: public
          cidr: "172.16.5.0/28"
      networkSecurityGroup:
        list:
          - name: db
            role: custom
            egressRules:
              - egressRule:
                  isStateless: false
                  destination: "172.16.5.0/28"
                  protocol: "6"
                  destinationType: "CIDR_BLOCK"
                  description: "All traffic to control plane nodes"
                  tcpOptions:
                    destinationPortRange:
                      max: 6443
                      min: 6443
```

[sl-vs-nsg]: https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/securityrules.htm#comparison
[externally-managed-cluster-infrastructure]: ../gs/externally-managed-cluster-infrastructure.md#example-spec-for-externally-managed-vcn-infrastructure
[oci-nlb]: https://docs.oracle.com/en-us/iaas/Content/NetworkLoadBalancer/introducton.htm#Overview
[oci-lb]: https://docs.oracle.com/en-us/iaas/Content/Balance/Concepts/balanceoverview.htm#Overview_of_Load_Balancing
