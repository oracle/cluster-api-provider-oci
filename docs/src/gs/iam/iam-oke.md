# Configure OCI policies for an Oracle Container Engine for Kubernetes cluster

These steps are applicable if you intend to run your management cluster using [Oracle Container Engine for Kubernetes][oke] (OKE). They need to be created by a user with admin privileges and are required so you can provision your OKE cluster successfully. If you plan to run your management cluster in [kind][kind] or a non-OKE cluster, you can skip this step.

1. [Create a user in OCI](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingusers.htm) e.g. `iaas_oke_usr`
1. [Create a group in OCI](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managinggroups.htm) e.g. `iaas_oke_grp` and add the user `iaas_oke_usr` to this group
1. Create a policy in OCI and add the following policies(Please read [OKE Policy Configuration Doc][oke-policy] for more fine grained policies):
   - `Allow group iaas_oke_grp to manage dynamic groups`
   - `Allow group iaas_oke_grp to manage virtual-network-family in <compartment>`
   - `Allow group iaas_oke_grp to manage cluster-family in <compartment>`
   - `Allow group iaas_oke_grp to manage instance-family in <compartment>`

where `<compartment>` is the name of the OCI compartment of the management cluster. Refer to the [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingcompartments.htm) if you have not created a compartment yet.

```admonish warning
You should not create your management cluster in the root compartment.
```


[kind]: https://kind.sigs.k8s.io/
[oke]: https://docs.oracle.com/en-us/iaas/Content/ContEng/home.htm
[oke-policy]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengpolicyconfig.htm
