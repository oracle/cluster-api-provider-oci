# Create a workload cluster

## Workload Cluster Templates

Choose one of the available templates for to create your workload clusters from the
[latest released artifacts][latest-release]. Each workload cluster template can be
further configured  with the parameters below.

## Workload Cluster Parameters

The following Oracle Cloud Infrastructure (OCI) configuration parameters are available
when creating a workload cluster on OCI using one of our predefined templates:

| Parameter                                 | Default Value       | Description                                                                                                                                                                                                                                                     |
|-------------------------------------------|---------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `OCI_COMPARTMENT_ID`                      |                     | The OCID of the compartment in which to create the required compute, storage and network resources.                                                                                                                                                             |
| `OCI_IMAGE_ID`                            |                     | The OCID of the image for the kubernetes nodes. This same image is used for both the control plane and the worker nodes.                                                                                                                                        |
| `OCI_CONTROL_PLANE_MACHINE_TYPE`          | VM.Standard.E4.Flex | The [shape](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm) of the Kubernetes control plane machine.                                                                                                                           |
| `OCI_CONTROL_PLANE_MACHINE_TYPE_OCPUS`    | 1                   | The number of OCPUs allocated to the control plane instance.                                                                                                                                                                                                    |
| `OCI_NODE_MACHINE_TYPE`                   | VM.Standard.E4.Flex | The [shape](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm) of the Kubernetes worker machine.                                                                                                                                  |
| `OCI_NODE_MACHINE_TYPE_OCPUS`             | 1                   | The number of OCPUs allocated to the worker instance.                                                                                                                                                                                                           |
| `OCI_SSH_KEY`                             |                     | The public SSH key to be added to the Kubernetes nodes. It can be used to login to the node and troubleshoot failures.                                                                                                                                          |
| `OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION` | true                | Enables [in-flight Transport Layer Security (TLS) 1.2 encryption](https://docs.oracle.com/en-us/iaas/Content/File/Tasks/intransitencryption.htm) of data between control plane nodes and their associated block storage devices.                                |
| `OCI_NODE_PV_TRANSIT_ENCRYPTION`          | true                | Enables [in-flight Transport Layer Security (TLS) 1.2 encryption](https://docs.oracle.com/en-us/iaas/Content/File/Tasks/intransitencryption.htm) of data between worker nodes and their associated block storage devices.                                       |

> Note: Only specific [bare metal shapes](https://docs.oracle.com/en-us/iaas/releasenotes/changes/60d602f5-abb3-4639-aa19-292a5744a808/)
support in-transit encryption. If an unsupported shape is specified, the deployment will fail completely.

> Note: Using the predefined templates the machine's memory size is automatically allocated based on the chosen shape 
and OCPU count.

The following Cluster API parameters are also available:

| Parameter                     | Default Value  | Description                                                                                                                                                                               |
|-------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `CLUSTER_NAME`                |                | The name of the workload cluster to create.                                                                                                                                               |
| `CONTROL_PLANE_MACHINE_COUNT` | 1              | The number of control plane machines for the workload cluster.                                                                                                                            |
| `KUBERNETES_VERSION`          |                | The Kubernetes version installed on the workload cluster nodes. If this environement variable is not configured, the version must be specifed in the `.cluster-api/clusterctl.yaml` file  |
| `NAMESPACE`                   |                | The namespace for the workload cluster. If not specified, the current namespace is used.                                                                                                  |
| `POD_CIDR`                    | 192.168.0.0/16 | CIDR range of the Kubernetes pod-to-pod network.                                                                                                                                          |
| `SERVICE_CIDR`                | 10.128.0.0/12  | CIDR range of the Kubernetes pod-to-services network.                                                                                                                                     |
| `NODE_MACHINE_COUNT`          |                | The number of worker machines for the workload cluster.                                                                                                                                   |

## Create a new workload cluster on virtual instances using an Ubuntu custom image

The following command will create a workload cluster comprising a single 
control plane node and single worker node using the default values as specified in the preceding 
[Workload Cluster Parameters](#workload-cluster-parameters) table:

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
CONTROL_PLANE_MACHINE_COUNT=1 \
KUBERNETES_VERSION=v1.20.10 \
NAMESPACE=default \
NODE_MACHINE_COUNT=1 \
clusterctl generate cluster <cluster-name>\
--from cluster-template.yaml | kubectl apply -f -
```

## Create a new workload cluster on bare metal instances using an Ubuntu custom image

The following command uses the `OCI_CONTROL_PLANE_MACHINE_TYPE` and `OCI_NODE_MACHINE_TYPE` 
parameters to specify bare metal shapes instead of using CAPOCI's default virtual 
instance shape. The `OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION` and `OCI_NODE_PV_TRANSIT_ENCRYPTION` 
parameters disable encryption of data in flight between the bare metal instance and the block storage resources.

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
OCI_CONTROL_PLANE_MACHINE_TYPE=BM.Standard2.52 \
OCI_CONTROL_PLANE_MACHINE_TYPE_OCPUS=52 \
OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION=false \
OCI_NODE_MACHINE_TYPE=BM.Standard2.52 \
OCI_NODE_MACHINE_TYPE_OCPUS=52 \
OCI_NODE_PV_TRANSIT_ENCRYPTION=false \
CONTROL_PLANE_MACHINE_COUNT=1 \
KUBERNETES_VERSION=v1.20.10 \
NAMESPACE=default \
WORKER_MACHINE_COUNT=1 \
clusterctl generate cluster <cluster-name>\
--from cluster-template.yaml| kubectl apply -f -
```

## Create a new workload cluster on virtual instances using an Oracle Linux custom image

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<oracle-linux-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
CONTROL_PLANE_MACHINE_COUNT=1 \
KUBERNETES_VERSION=v1.20.10 \
NAMESPACE=default \
WORKER_MACHINE_COUNT=1 \
clusterctl generate cluster <cluster-name>\
--from cluster-template-oraclelinux.yaml | kubectl apply -f -
```

## Create a workload cluster in an alternative region

CAPOCI provides a way to launch and manage your workload cluster in multiple
regions. Choose the `cluster-template-alternative-region.yaml` template when
creating your workload clusters from the [latest released artifacts][latest-release].
Currently, the other templates do not support the ability to change the workload
cluster region. 

Each cluster can be further configured with the parameters
defined in [Workload Cluster Parameters](#workload-cluster-parameters) and
additionally with the parameter below.

| Parameter             | Default Value                                        | Description                                                                                                                        |
|-----------------------|------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| `OCI_WORKLOAD_REGION` | Configured as [`OCI_REGION`][configure-authentication] | The [OCI region](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm) in which to launch the workload cluster. |

The following example configures the CAPOCI provider to authenticate in
`us-phoenix-1` and launch a workload cluster in `us-sanjose-1`.

> Note: Ensure the specified image is available in your chosen region or the launch will fail.

To configure authentication for management cluster, follow the steps in 
[Configure authentication][configure-authentication].

Extend the preceding configuration with the following additional configuration
parameter and initialize the CAPOCI provider.

```bash
...
export OCI_REGION=us-phoenix-1
...
   
clusterctl init --infrastructure oci
```

Create a new workload cluster in San Jose (`us-sanjose-1`) by explicitly setting the
`OCI_WORKLOAD_REGION` environment variable when invoking `clusterctl`:

```bash
OCI_WORKLOAD_REGION=us-sanjose-1 \
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<in-region-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
CONTROL_PLANE_MACHINE_COUNT=1 \
KUBERNETES_VERSION=v1.20.10 \
NAMESPACE=default \
NODE_MACHINE_COUNT=1 \
clusterctl generate cluster <cluster-name>\
--from cluster-template-alternative-region.yaml | kubectl apply -f -
```

### Access workload cluster Kubeconfig

Execute the following command to list all the workload clusters present:

```bash
kubectl get clusters -A
```

Execute the following command to access the kubeconfig of a workload cluster:

```bash
clusterctl get kubeconfig <cluster-name> -n default > <cluster-name>.kubeconfig
```

### Install a CNI Provider

After creating a workload cluster, a [CNI][cni] provider must be installed in the workload cluster. Until you install a
a [CNI][cni] provider, the cluster nodes will not go into the `Ready` state.

For example, you can install [Calico][calico] as follows:

```bash
kubectl --kubeconfig=<cluster-name>.kubeconfig \
  apply -f https://docs.projectcalico.org/v3.21/manifests/calico.yaml
```

You can use your preferred CNI provider. Currently, the following providers have been tested and verified to work:

| CNI               | CNI Version   | Kubernetes Version | CAPOCI Version |
| ----------------- | --------------| ------------------ | -------------- |
| [Calico][calico]  |     3.21      |     1.20.10        |   0.1          |
| [Antrea][antrea]  |               |     1.20.10        |   0.1          |

If you have tested an alternative CNI provider and verified it to work, please send us a PR to add it to the list.

If you have an issue with your alternative CNI provider, please raise an issue on GitHub.

### Install OCI Cloud Controller Manager and CSI in a self-provisioned cluster

By default, the [OCI Cloud Controller Manager (CCM)][oci-ccm] is not installed into a workload cluster. To install the OCI CCM, follow [these instructions][install-oci-ccm].

[antrea]: ../networking/antrea.md
[calico]: ../networking/calico.md
[cni]: https://www.cni.dev/
[oci-ccm]: https://github.com/oracle/oci-cloud-controller-manager
[latest-release]: https://github.com/oracle/cluster-api-provider-oci/releases/tag/v0.4.0
[install-oci-ccm]: ./install-oci-ccm.md
[configure-authentication]: ./install-cluster-api.html#configure-authentication
