# Provision a management cluster with Oracle Container Engine for Kubernetes

For this release, if you use [Oracle Container Engine for Kubernetes][oke] (OKE) for your management cluster, you will be provisioning a public Kubernetes cluster i.e. its API server must be accessible to `kubectl`. You can use either use the OCI console to do the provisioning or the [terraform-oci-oke][terraform-oci-oke] project.

1. Login to the OCI Console as the `iaas_oke_usr`

1. Search for OKE and select it:

   ![OKE](../../images/oke_1.png)

1. Select the right compartment where you will be creating the OKE Cluster:

   ![OKE](../../images/oke_2.png)

1. Click **Create Cluster**, select **Quick Create** and click **Launch Workflow**:

   ![OKE](../../images/oke_3.png)

1. Name your cluster and ensure you select **Public Endpoint** and choose **Private Workers**:

   ![OKE](../../images/oke_4.png)

1. Click **Next** and **Create Cluster**.

1. When the cluster is ready, set up access to the OKE cluster. You can either use
   - [OCI Cloud Shell](https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengdownloadkubeconfigfile.htm#cloudshelldownload)
   - or [your local terminal](https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengdownloadkubeconfigfile.htm#localdownload).

> If you are working with an existing Kubernetes cluster and have an existing `kubeconfig` in your `$HOME/.kube/config` directory, running the command to set up local access will overwrite your existing `kubeconfig`.

[management_cluster]: https://cluster-api.sigs.k8s.io/user/concepts.html#management-cluster
[oke]: https://docs.oracle.com/en-us/iaas/Content/ContEng/home.htm
[terraform-oci-oke]: https://github.com/oracle-terraform-modules/terraform-oci-oke
