# Self managed nodes
CAPOCI supports [OKE self managed nodes][self-managed-nodes]. With this feature, CAPI features such as [rolling update][capi-upgrade],
[health checks][health-check] can be used to make management of self managed nodes easier. CAPOCI supports two
flavours fo self managed nodes. It also allows full range of [OCI Compute API][oci-compute-api] to be used while 
creating worker nodes.

Please read the prerequisites related to [Dynamic Group and policy][policy] before creating self managed OKE nodes.

# Self managed nodes backed by CAPI Machine Deployment
Use the template `cluster-template-managed-self-managed-nodes.yaml` as an example for creating and OKE cluster
with a self managed CAPI machine deployment. Self managed nodes are only supported if flannel
is used as CNI provider. The following snippet shows the relevant part of the template.
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCIMachineTemplate
metadata:
  name: "${CLUSTER_NAME}-md-0"
spec:
  template:
    spec:
      imageId: "${OCI_MANAGED_NODE_IMAGE_ID}"
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: "${CLUSTER_NAME}-md-0"
spec:
  clusterName: "${CLUSTER_NAME}"
  replicas: ${WORKER_MACHINE_COUNT}
  selector:
    matchLabels:
  template:
    spec:
      clusterName: "${CLUSTER_NAME}"
      version: "${KUBERNETES_VERSION}"
      bootstrap:
        dataSecretName: "${CLUSTER_NAME}-self-managed"
      infrastructureRef:
        name: "${CLUSTER_NAME}-md-0"
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
        kind: OCIMachineTemplate
```
Note that CAPOCI will populate a bootstrap secret with the relevant [cloud-init][cloud-init] script required 
for the node to join the OKE cluster

# Self managed nodes backed by OCI Instance Pool
> Note: MachinePool is still an experimental feature in CAPI

The following snippet can be used to create self managed OKE nodes backed by OCI Instance Pool[instance-pool].
```yaml
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
metadata:
  name: "${CLUSTER_NAME}-mp-0"
  namespace: default
spec:
  clusterName: "${CLUSTER_NAME}"
  replicas: "${WORKER_MACHINE_COUNT}"
  template:
    spec:
      bootstrap:
        dataSecretName: "${CLUSTER_NAME}-self-managed"
      clusterName: "${CLUSTER_NAME}"
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
        kind: OCIMachinePool
        name: "${CLUSTER_NAME}-mp-0"
      version: "${KUBERNETES_VERSION}"
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: OCIMachinePool
metadata:
  name: "${CLUSTER_NAME}-mp-0"
  namespace: default
spec:
  instanceConfiguration:
    metadata:
      ssh_authorized_keys: "${OCI_SSH_KEY}"
    instanceSourceViaImageConfig:
      imageId: "${OCI_MANAGED_NODE_IMAGE_ID}"
    shape: "${OCI_NODE_MACHINE_TYPE=VM.Standard.E4.Flex}"
    shapeConfig:
      ocpus: "1"
```




[self-managed-nodes]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengworkingwithselfmanagednodes.htm
[capi-upgrade]:https://cluster-api.sigs.k8s.io/tasks/upgrading-clusters.html#upgrading-machines-managed-by-a-machinedeployment
[health-check]: https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking.html
[oci-compute-api]: https://docs.oracle.com/en-us/iaas/api/#/en/iaas/20160918/Instance/LaunchInstance
[cloud-init]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengcloudinitforselfmanagednodes.htm
[policy]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengprereqsforselfmanagednodes.htm
[instance-pool]: https://docs.oracle.com/en-us/iaas/Content/Compute/Concepts/instancemanagement.htm#Instance