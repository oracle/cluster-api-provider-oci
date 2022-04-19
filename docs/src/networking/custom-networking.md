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
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
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
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    vcn:
      name: ${CLUSTER_NAME}
      cidr: "172.16.0.0/16"
      networkSecurityGroups:
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
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
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

CAPOCI can be used to create a cluster using existing VCN infrastructure. In this case, only the
API Server Load Balancer will be managed by CAPOCI.

Example spec is given below

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    skipNetworkManagement: true
    vcn:
      id: <Insert VCN OCID Here>
      networkSecurityGroups:
        - id: <Insert Control Plane Endpoint NSG OCID Here>
          role: control-plane-endpoint
        - id: <Insert Worker NSG OCID Here>
          role: worker
        - id: <Insert Control Plane NSG OCID Here>
          role: control-plane
      subnets:
        - id: <Insert Control Plane Endpoint Subnet OCID Here>
          role: control-plane-endpoint
        - id: <Insert Worker Subnet OCID Here>
          role: worker
        - id: <Insert control Plane Subnet OCID Here>
          role: control-plane
```

[sl-vs-nsg]: https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/securityrules.htm#comparison
