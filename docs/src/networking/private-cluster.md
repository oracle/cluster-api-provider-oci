# Using private clusters

> Note: This section has to be used only if the CAPOCI manages the workload cluster VCN. If externally managed VCN is
> used, this section is not applicable.

## Example Spec for private cluster

### Using networkVisibility (recommended)

CAPOCI v0.25.0+ exposes `spec.networkSpec.apiServerLoadBalancer.networkVisibility` to control API server load balancer visibility (public or private). The default `Inherited` value preserves the legacy behavior of inferring visibility from the control plane endpoint subnet.

| Value      | Description |
|------------|-------------|
| `Inherited` (default) | Matches the control-plane-endpoint subnet type, maintaining backwards compatibility. |
| `Private`  | Creates a private load balancer reachable only within the VCN or peered VCNs. |
| `Public`   | Creates a public load balancer with a public IP address. |

> NOTE: The webhook rejects `networkVisibility: Public` when any control-plane-endpoint subnet is marked `type: private`, because OCI does not allow attaching a public load balancer to a private subnet.
>
> OCI does not support changing an existing API server load balancer between private and public visibility. Choose the desired visibility before creating the cluster.

```yaml
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    apiServerLoadBalancer:
      networkVisibility: Private
```

The `templates/cluster-template-local-vcn-peering.yaml` asset already includes this setting and can be used as a starting point.

### Using subnet type (legacy approach)

Before `networkVisibility` existed, CAPOCI inferred the API server exposure from the control-plane-endpoint subnet type. This workflow remains supported for backwards compatibility and is functionally equivalent to setting `networkVisibility: Inherited`.

CAPOCI supports private clusters where the Kubernetes API server endpoint is a private IP address and is accessible only within the VCN or peered VCNs. In order to use private clusters with the legacy mechanism, the control-plane-endpoint subnet has to be marked as private. An example spec is given below.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta3
kind: OCICluster
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: "${CLUSTER_NAME}"
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
  networkSpec:
    vcn:
      subnets:
        - cidr: 10.1.0.8/29
          name: control-plane-endpoint
          role: control-plane-endpoint
          type: private
        - cidr: 10.1.0.0/29
          name: control-plane
          role: control-plane
          type: private
        - cidr: 10.1.0.32/27
          name: service-lb
          role: service-lb
          type: public
        - cidr: 10.1.64.0/20
          name: worker
          role: worker
          type: private
```

When using the legacy approach (or `networkVisibility: Inherited`), the API server endpoint visibility is derived from the control-plane-endpoint subnet type — ensure it is marked `type: private` to keep the API server address internal.

## Example spec for VCN Peering using Dynamic Routing Gateway (Local)

While using private clusters, the management cluster needs to talk to the workload cluster. If the
management cluster and workload cluster are in separate VCN, the VCN peering can be used to connect the management
and workload cluster VCNS. CAPOCI supports peering of the workload cluster VCN with another VCN in the same region using
[Dynamic Routing Gateway][drg].

In case of local VCN peering, a DRG OCID has to be provided and CAPOCI will attach the workload cluster VCN to the
provided DRG. The recommendation is to attach the management cluster VCN also to the same DRG so that the VCNs are
peered to each other. For more details see [Local VCN Peering using Local Peering Gateways][drg-local].

An example template for this `cluster-template-local-vcn-peering.yaml` can be found in the Assets section under the
 [CAPOCI release page][capi-latest-release]. 

In order to use the template, the following Cluster API parameters have to be set in addition to the common parameters 
explained in the [Workload Cluster Parameters table][common].

| Parameter | Default Value | Description                                                     |
|-----------|---------------|-----------------------------------------------------------------|
| `DRG_ID`  |               | OCID of the DRG to which the worklaod cluster will be attached. |


## Example spec for VCN Peering using Dynamic Routing Gateway (Remote)

If the management cluster and workload cluster are in different OCI regions, then DRG can still be used. In this case,
in addition to VCN attachment, [Remote Peering Connection (RPC) ][drg-rpc] has to be used.

In case of remote VCN peering, a DRG will be created by CAPOCI, and the workload cluster VCN will be attached to the
DRG. In addition, a remote DRG has to be provided. CAPOCI will create RPC in the local and remote VCN and
connection will be established between the RPCs.

An example template for this `cluster-template-remote-vcn-peering.yaml` can be found in the Assets section under the
[CAPOCI release page][capi-latest-release]. 

In order to use the template, the following Cluster API parameters have to be set in addition to the common parameters
explained in the [Workload Cluster Parameters table][common]. Typically, the peer DRG refers to the DRG to 
which the management cluster VCN is attached.

| Parameter          | Default Value | Description                                                 |
|--------------------|---------------|-------------------------------------------------------------|
| `PEER_DRG_ID`      |               | OCID of the peer DRG to which the local DRG will be peered. |
| `PEER_REGION_NAME` |               | The region to which the peer DRG belongs.                   |

[common]: ../gs/create-workload-cluster.md#workload-cluster-parameters
[drg]: https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/managingDRGs.htm
[drg-local]: https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/localVCNpeering.htm
[drg-rpc]: https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/scenario_e.htm
[capi-latest-release]: https://github.com/oracle/cluster-api-provider-oci/releases/latest
