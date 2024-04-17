# Managed Clusters (OKE)

Cluster API Provider for OCI (CAPOCI) supports managing OCI Container 
Engine for Kubernetes (OKE) clusters. CAPOCI implements this with three
custom resources:
- `OCIManagedControlPlane`
- `OCIManagedCluster`
- `OCIManagedMachinePool`

## Workload Cluster Parameters

The following Oracle Cloud Infrastructure (OCI) configuration parameters are available
when creating a managed workload cluster on OCI using one of our predefined templates:

| Parameter                             | Mandatory | Default Value       | Description                                                                                                                                                                                            |
|---------------------------------------|-----------|---------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `OCI_COMPARTMENT_ID`                  | Yes       |                     | The OCID of the compartment in which to create the required compute, storage and network resources.                                                                                                    |
| `OCI_MANAGED_NODE_IMAGE_ID`           | No        | ""                  | The OCID of the image for the Kubernetes worker nodes. Please read the [doc][node-images] for more details.  If no value is specified, a default Oracle Linux OKE platform image is looked up and used |
| `OCI_MANAGED_NODE_SHAPE `             | No        | VM.Standard.E4.Flex | The [shape][node-images-shapes] of the Kubernetes worker nodes.                                                                                                                                        |
| `OCI_MANAGED_NODE_MACHINE_TYPE_OCPUS` | No        | 1                   | The number of OCPUs allocated to the worker node instance.                                                                                                                                             |
| `OCI_SSH_KEY`                         | Yes       |                     | The public SSH key to be added to the Kubernetes nodes. It can be used to login to the node and troubleshoot failures.                                                                                 |

> Note: In production use-case, the node pool image id must be provided explicitly and the default lookup mechanism must not be used.
> 
The following Cluster API parameters are also available:

| Parameter                     | Default Value  | Description                                                                                                                                                                               |
|-------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `CLUSTER_NAME`                |                | The name of the workload cluster to create.                                                                                                                                               |
| `KUBERNETES_VERSION`          |                | The Kubernetes version installed on the workload cluster nodes. If this environement variable is not configured, the version must be specified in the `.cluster-api/clusterctl.yaml` file |
| `NAMESPACE`                   |                | The namespace for the workload cluster. If not specified, the current namespace is used.                                                                                                  |
| `NODE_MACHINE_COUNT`          |                | The number of machines in the OKE nodepool.                                                                                                                                               |


## Pre-Requisites

### OCI Security Policies

Please read the [doc][oke-policies] and add the necessary policies required for the user group.
Please add the policies for dynamic groups if instance principal is being used as authentication 
mechanism. Please read the [doc][install-cluster-api] to know more about authentication mechanisms.

## Workload Cluster Templates

Choose one of the available templates to create your workload clusters from the
[latest released artifacts][latest-release]. The managed cluster templates is of the
form `cluster-template-managed-<flavour>`.yaml . The default managed template is
`cluster-template-managed.yaml`. Please note that the templates provided are to be considered
as references and can be customized further as the [CAPOCI API Reference][api-reference].

## Supported Kubernetes versions
The [doc][supported-versions] lists the Kubernetes versions currently supported by OKE.

## Create a new OKE cluster.

The following command will create an OKE cluster using the default template. The created node pool uses 
[VCN native pod networking][vcn-native-pod-networking].

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_MANAGED_NODE_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
KUBERNETES_VERSION=v1.24.1 \
NAMESPACE=default \
clusterctl generate cluster <cluster-name>\
--from cluster-template-managed.yaml | kubectl apply -f -
```

## Create a new private OKE cluster.

The following command will create an OKE private cluster. In this template, the control plane endpoint subnet is a
private subnet and the API endpoint is accessible only within the subnet. The created node pool uses
[VCN native pod networking][vcn-native-pod-networking].

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_MANAGED_NODE_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
KUBERNETES_VERSION=v1.24.1 \
NAMESPACE=default \
clusterctl generate cluster <cluster-name>\
--from cluster-template-managedprivate.yaml | kubectl apply -f -
```

## Access kubeconfig of an OKE cluster

###  Step 1 - Identify Cluster OCID

Access the management cluster using kubectl and identify the OKE cluster OCID

```bash
kubectl get ocimanagedcontrolplane <workload-cluster-name> -n <workload-cluster-namespace> -o jsonpath='{.spec.id}'
```

###  Step 2 - Access kubeconfig

Access kubeconfig

```bash
oci ce cluster create-kubeconfig --cluster-id <cluster-ocid> --file <file-name>  --region <region>  --kube-endpoint PUBLIC_ENDPOINT
```
Please read the [doc][download-kubeconfig] for more details on how to access kubeconfig file of OKE clusters.


[node-images]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Reference/contengimagesshapes.htm#images
[node-images-shapes]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Reference/contengimagesshapes.htm#shapes
[oke-policies]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengpolicyconfig.htm
[install-cluster-api]: ../gs/install-cluster-api.md
[latest-release]: https://github.com/oracle/cluster-api-provider-oci/releases/latest
[api-reference]: ../reference/api-reference.md
[supported-versions]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengaboutk8sversions.htm#supportedk8sversions
[vcn-native-pod-networking]: https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengpodnetworking_topic-OCI_CNI_plugin.htm
[download-kubeconfig]:https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengdownloadkubeconfigfile.htm