# Self managed nodes
CAPOCI supports OKE self managed nodes. With this feature, CAPI features such as [rolling update][capi-upgrade],
[health checks][health-check] can be used to make management of self managed nodes easier. CAPOCI supports two
flavours fo self managed nodes. It also allows full range of [OCI Compute API][oci-compute-api] to be used while 
creating worker nodes.

# Self managed nodes backed by CAPI Machine Deployment
Use the template `cluster-template-managed-self-managed-nodes.yaml` as an example for creating

# Self managed nodes backed by OCI Instance Pool




[self-managed-nodes]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengworkingwithselfmanagednodes.htm
[capi-upgrade]:https://cluster-api.sigs.k8s.io/tasks/upgrading-clusters.html#upgrading-machines-managed-by-a-machinedeployment
[health-check]: https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking.html
[oci-compute-api]: https://docs.oracle.com/en-us/iaas/api/#/en/iaas/20160918/Instance/LaunchInstance