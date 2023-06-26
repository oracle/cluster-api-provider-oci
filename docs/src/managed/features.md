# Features

This page will cover configuration of various OKE features in CAPOCI.

## Node Pool Cycling
OKE [Node Pool Cycling][node-pool-cycling] can be used during Kubernetes version upgrades to cycle
the nodes such that all the nodes of a Node Pool is running on the newer Kubernetes version. The following
`OCIManagedMaachinePool` spec can be used to specify Node Pool cycling option.
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCIManagedMachinePool
spec:
  nodePoolCyclingDetails:
    isNodeCyclingEnabled: true
```

## Addons
The following `OCIManagedControlPlane` spec can be used to specify [Addons][addons] which has to be
installed in the OKE cluster.

```yaml
kind: OCIManagedControlPlane
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
spec:
  clusterType: "ENHANCED_CLUSTER"
  addons:
  - name: CertManager
```
More details about the configuration parameters is available in [CAPOCI API Reference][api-reference].


[node-pool-cycling]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengupgradingk8sworkernode_topic-Performing_an_InPlace_Worker_Node_Upgrade_by_Cycling_an_Existing_Node_Pool.htm#contengupgradingk8sworkernode_topic-Performing_an_InPlace_Worker_Node_Upgrade_by_Cycling_an_Existing_Node_Pool
[addons]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengconfiguringclusteraddons.htm
[api-docs]: ../reference/api-reference.md