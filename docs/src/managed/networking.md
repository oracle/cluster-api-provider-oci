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

## Use flannel as CNI

Use the template `cluster-template-managed-flannel.yaml` as an example for using flannel as the CNI. The template
sets the correct parameters in the spec as well as create the proper security roles in the Network Security Group (NSG).
