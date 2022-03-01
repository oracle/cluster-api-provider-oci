# Configure policies for a self-provisioned cluster

Although some policies required for Oracle Container Engine for Kubernetes (OKE) and self-provisioned clusters may overlap, we recommend you create another user and group for the principal that will be provisioning the self-provisioned clusters.

1. [Create a user in OCI](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingusers.htm) e.g. `cluster_api_usr`
1. [Create a group in OCI](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managinggroups.htm) e.g. `cluster_api_grp` and add the user `cluster_api_usr` to this group
1. Create a policy in OCI and add the following policies:
   - `Allow group cluster_api_grp to manage virtual-network-family in <compartment>`
   - `Allow group cluster_api_grp to manage load-balancers in <compartment>`
   - `Allow group cluster_api_grp to manage instance-family in <compartment>`

where `<compartment>` is the name of the OCI compartment of the workload cluster. Your workload compartment may be different from the management compartment. Refer to the [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingcompartments.htm) if you have not created a compartment yet.

```admonish info
If you are an administrator and you are experimenting with CAPOCI, you can skip creating the policies.
```

1. Repeat the procedure as for the `iaas_oke_usr` above to obtain the IAM details.

```admonish warning
You should not create your workload cluster in the root compartment.
```

[kind]: https://kind.sigs.k8s.io/
[oke]: https://docs.oracle.com/en-us/iaas/Content/ContEng/home.htm
