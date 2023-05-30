# OKE Enhanced Clusters and Virtual Nodes

CAPOCI supports OKE [Enhanced Clusters][enhanced-cluster] and [Virtual Nodes][virtual-node-pool]. A cluster-template 
`cluster-template-managed-virtual-node.yaml` with Enhanced Cluster and Virtual Node Pool has been released in 
CAPOCI release artifacts which can be referred using  the flavor `managed-virtual-node` in `clusterctl generate` 
command.


## Create Enhanced Cluster
The following `OCIManagedControlPlane` snippet can be used to create an enhanced OKE cluster.

```yaml
kind: OCIManagedControlPlane
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
spec:
  clusterType: "ENHANCED_CLUSTER"
```

## Create Virtual Node Pool

The following `OCIVirtualMachinePool` snippet can be used to create a Virtual Node Pool. Please read through [CAPOCI 
API Docs][api-docs] to see all the supported parameters of `OCIVirtualMachinePool`.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCIVirtualMachinePool
spec:
```

[enhanced-cluster]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengcomparingenhancedwithbasicclusters_topic.htm
[virtual-node-pool]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengworkingwithvirtualnodes.htm
[api-docs]: ../reference/api-reference.md
