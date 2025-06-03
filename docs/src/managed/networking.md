# Networking customizations 
## Use a pre-existing VCN

The following `OCIManagedCluster` snippet can be used to to use a pre-existing VCN.

```yaml
kind: OCIManagedCluster
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    skipNetworkManagement: true
    vcn:
      id: "<vcn-id>"
      networkSecurityGroup:
        list:
          - id: "<control-plane-endpoint-nsg-id>"
            role: control-plane-endpoint
            name: control-plane-endpoint
          - id:  "<worker-nsg-id>"
            role: worker
            name: worker
          - id: "<pod-nsg-id>"
            role: pod
            name: pod
      subnets:
        - id: "<control-plane-endpoint-subnet-id>"
          role: control-plane-endpoint
          name: control-plane-endpoint
          type: public
        - id: "<worker-subnet-id>"
          role: worker
          name: worker
        - id: "<pod-subnet-id>"
          role: pod
          name: pod
        - id: "<service-lb-subnet-id>"
          role: service-lb
          name: service-lb
          type: public
```

## Use a pre-existing VCN, Subnet and Gatways, but the other networking components self managed

The following `OCIManagedCluster` example spec is given below

```yaml
kind: OCIManagedCluster
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    skipNetworkManagement: true
    vcn:
      skip: true
      id: <Insert VCN OCID Here>
      networkSecurityGroup:
        skip: false
      internetGateway:
        skip: true
      natGateway:
        skip: true
      serviceGateway:
        skip: true
      routeTable:
        skip: true
      subnets:
        - id: <Insert control Plane Subnet OCID Here>
          role: control-plane-endpoint
          name: control-plane-endpoint
          type: public
          skip: true
        - id: <Insert control Plane Subnet OCID Here>
          role: worker
          name: worker
          type: private
          skip: true
        - id: <Insert control Plane Subnet OCID Here>
          role: control-plane
          name: control-plane
          type: private
          skip: true
        - id: <Insert control Plane Subnet OCID Here>
          role: service-lb
          name: service-lb
          type: public
          skip: true
```

## Use flannel as CNI

Use the template `cluster-template-managed-flannel.yaml` as an example for using flannel as the CNI. The template
sets the correct parameters in the spec as well as create the proper security roles in the Network Security Group (NSG).
