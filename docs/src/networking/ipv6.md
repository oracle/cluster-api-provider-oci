# IPv6 Global Unicast Address (GUA) Allocated by Oracle Guide

This section contains information about the IPv6 aspects of Cluster API Provider OCI.

In order to have a VM created with IPv6 Ip assigned you should have the following defined:
1. For OCICluster:
    - ``` yaml 
        networkSpec:
            vcn:
                isIpv6Enabled: true
                subnets:
                    - ipv6CidrBlockHextet: "01"
      ```
* example:
```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: "${CLUSTER_NAME}"
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    vcn:
      isIpv6Enabled: true
      subnets:
        - ipv6CidrBlockHextet: "01"
          name: control-plane-endpoint
          role: control-plane-endpoint
          type: public
        - ipv6CidrBlockHextet: "02"
          name: control-plane
          role: control-plane
          type: private
        - ipv6CidrBlockHextet: "03"
          name: service-lb
          role: service-lb
          type: public
        - ipv6CidrBlockHextet: "04"
          name: worker
          role: worker
          type: private
```

2. For OCIMachineTemplate:
    - ``` yaml 
        networkDetails:
            assignIpv6Ip: true
      ```

* example: 
```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCIMachineTemplate
metadata:
  name: "${CLUSTER_NAME}-md-0"
spec:
  template:
    spec:
      imageId: "${OCI_IMAGE_ID}"
      compartmentId: "${OCI_COMPARTMENT_ID}"
      shape: "${OCI_NODE_MACHINE_TYPE=VM.Standard.E5.Flex}"
      shapeConfig:
        ocpus: "${OCI_NODE_MACHINE_TYPE_OCPUS=1}"
      metadata:
        ssh_authorized_keys: "${OCI_SSH_KEY}"
      isPvEncryptionInTransitEnabled: ${OCI_NODE_PV_TRANSIT_ENCRYPTION=true}
      networkDetails:
        assignIpv6Ip: true
```

- [IPv6 Example Template](../../../templates/cluster-template-with-ipv6.yaml)


**Note:**
- **VCNs, subnets and VMs can be asigned with IPv6 just at creation step. After creation they can be updated with IPv6 just from UI console.**
- **You cannot create a VM with IPv6 in a subnet/vcn without IPv6 enabled capabilities.**
- **Once VCNs, subnets are enabled with IPv6 cannot unassign their IPv6 CIDRs.**
- **CAPOCI don't support BYOIP IPv6 prefix at the moment.**
