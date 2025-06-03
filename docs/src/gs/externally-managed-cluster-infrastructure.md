# Externally managed Cluster infrastructure

By default, Cluster API will create resources on Oracle Cloud Infrastructure (OCI) when instantiating a new workload cluster. However, it is possible to have Cluster API re-use an existing OCI infrastructure instead of creating a new one. The existing infrastructure could include:

 1. Virtual cloud networks (VCNs)
 1. Network load balancers used as Kubernetes API Endpoint

## Example spec for externally managed VCN infrastructure

CAPOCI can be used to create a cluster using existing VCN infrastructure. In this case, only the
API Server Load Balancer will be managed by CAPOCI.

Example spec is given below

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    skipNetworkManagement: true
    vcn:
      id: <Insert VCN OCID Here>
      networkSecurityGroup:
        list:
          - id: <Insert Control Plane Endpoint NSG OCID Here>
            role: control-plane-endpoint
            name: control-plane-endpoint
          - id: <Insert Worker NSG OCID Here>
            role: worker
            name: worker
          - id: <Insert Control Plane NSG OCID Here>
            role: control-plane
            name: control-plane
      subnets:
        - id: <Insert Control Plane Endpoint Subnet OCID Here>
          role: control-plane-endpoint
          name: control-plane-endpoint
        - id: <Insert Worker Subnet OCID Here>
          role: worker
          name: worker
        - id: <Insert control Plane Subnet OCID Here>
          role: control-plane
          name: control-plane
```

In the above spec, note that name has to be mentioned for Subnet/NSG. This is so that Kubernetes
can merge the list properly when there is an update.



## Example spec for externally managed VCN, Subnet and Gateways, but the other networking components self managed


Example spec is given below

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
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

## Example `OCICluster` Spec with external infrastructure

CAPOCI supports [externally managed cluster infrastructure](https://github.com/kubernetes-sigs/cluster-api/blob/10d89ceca938e4d3d94a1d1c2b60515bcdf39829/docs/proposals/20210203-externally-managed-cluster-infrastructure.md).

If the `OCICluster` resource includes a `cluster.x-k8s.io/managed-by` annotation, then the [controller will skip any reconciliation](https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure.html#normal-resource).

This is useful for scenarios where a different persona is managing the cluster infrastructure out-of-band while still wanting to use CAPOCI for automated machine management.

The following `OCICluster` Spec includes the mandatory fields to be specified for externally managed infrastructure to work properly. In this example neither the VCN nor the network load balancer will be managed by CAPOCI.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCICluster
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: "${CLUSTER_NAME}"
  annotations:
    cluster.x-k8s.io/managed-by: "external"
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  controlPlaneEndpoint:
    host: <Control Plane Endpoint Address should go here>
    port: 6443
  networkSpec:
    apiServerLoadBalancer:
      loadBalancerId: <OCID of Control Plane Endpoint LoadBalancer>
    vcn:
      id: <OCID of VCN>
      networkSecurityGroup:
        list:
          - id: <OCID of Control Plane NSG>
            name: <Name of Control Plane NSG>
            role: control-plane
          - id: <OCID of Worker NSG>
            name: <Name of Worker NSG>
            role: worker
      subnets:
        - id: <OCID of Control Plane Subnet>
          role: control-plane
        - id: <OCID of Worker Subnet>
          role: worker
```



## Status

As per the Cluster API Provider specification, the `OCICluster Status` Object has to be updated with `ready` status
as well as the failure domain mapping. This has to be done after the `OCICluster` object has been created in the management cluster.
The following cURL request illustrates this:

Get a list of [Availability Domains](../reference/glossary.md#ad) of the region:

```bash
oci iam availability-domain list
```

```admonish info
Review the [OCI CLI documentation](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/cliconcepts.htm) for more information regarding this tool.
```

For 1-AD regions, use the following cURL command to update the status object:

```bash
curl -o  -s -X PATCH -H "Accept: application/json, */*" \
-H "Content-Type: application/merge-patch+json" \
--cacert ca.crt \
--cert client.crt \
--key client.key \
https://<management-plane-api-endpoint>/apis/infrastructure.cluster.x-k8s.io/v1beta2/namespaces/<cluster-namespace>/ociclusters/<cluster-name>/status \
--data '{"status":{"ready":true,"failureDomains":{"1":{"attributes":{"AvailabilityDomain":"zkJl:AP-HYDERABAD-1-AD-1","FaultDomain":"FAULT-DOMAIN-1"},"controlPlane":true},"2":{"attributes":{"AvailabilityDomain":"zkJl:AP-HYDERABAD-1-AD-1","FaultDomain":"FAULT-DOMAIN-2"},"controlPlane":true},"3":{"attributes":{"AvailabilityDomain":"zkJl:AP-HYDERABAD-1-AD-1","FaultDomain":"FAULT-DOMAIN-3"}}}}}'
```

For 3-AD regions, use the following cURL command to update the status object:

```bash
curl -o  -s -X PATCH -H "Accept: application/json, */*" \
-H "Content-Type: application/merge-patch+json" \
--cacert ca.crt \
--cert client.crt \
--key client.key \
https://<management-plane-api-endpoint>/apis/infrastructure.cluster.x-k8s.io/v1beta2/namespaces/<cluster-namespace>/ociclusters/<cluster-name>/status \
--data '{"status":{"ready":true,"failureDomains":{"1":{"attributes":{"AvailabilityDomain":"zkJl:US-ASHBURN-1-AD-1"},"controlPlane":true},"2":{"attributes":{"AvailabilityDomain":"zkJl:US-ASHBURN-1-AD-2"},"controlPlane":true},"3":{"attributes":{"AvailabilityDomain":"zkJl:US-ASHBURN-1-AD-3"}}}}}'
```

