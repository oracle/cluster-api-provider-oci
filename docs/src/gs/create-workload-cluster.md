# Create a workload cluster

## Workload Cluster Templates

Choose one of the available templates for to create your workload clusters from the [latest released artifacts][latest-release]. Each workload cluster template can be further configured with the parameters below.

## Workload Cluster Parameters

The following Oracle Cloud Infrastructure (OCI) configuration parameters are available when creating a workload cluster on OCI:

| Parameter                                 | Default Value       | Description                                                                                                                                                                                                                                                                                                                                                                                                    |
|-------------------------------------------|---------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `OCI_COMPARTMENT_ID`                      |                     | The OCID of the compartment where the OCI resources are to be created                                                                                                                                                                                                                                                                                                                                          |
| `OCI_IMAGE_ID`                            |                     | The OCID of the Compute Image (Oracle Linux or <br/>Ubuntu) with which to create the Kubernetes nodes. This same image is used for both the control plane and the worker nodes.                                                                                                                                                                                                                                |
| `OCI_CONTROL_PLANE_SHAPE`                 | VM.Standard.E4.Flex | The shape of the Kubernetes nodes                                                                                                                                                                                                                                                                                                                                                                              |
| `OCI_CONTROL_PLANE_SHAPE_MEMORY_IN_GBS`   |                     | The amount of memory in GBs to be allocated to the control plane instances. If not provided, the memory is automatically computed by compute API.                                                                                                                                                                                                                                                              |
| `OCI_CONTROL_PLANE_SHAPE_OCPUS`           | 1                   | The number of OCPUs allocated to the control plane instance                                                                                                                                                                                                                                                                                                                                                    |
| `OCI_WORKER_SHAPE`                        | VM.Standard.E4.Flex | The shape of the Kubernetes worker nodes                                                                                                                                                                                                                                                                                                                                                                       |
| `OCI_WORKER_SHAPE_MEMORY_IN_GBS`          |                     | The amount of memory in GBs to be allocated to the worker instances. If not provided, the memory is automatically computed by compute API.                                                                                                                                                                                                                                                                     |
| `OCI_WORKER_SHAPE_OCPUS`                  | 1                   | The number of OCPUs allocated to the worker instance                                                                                                                                                                                                                                                                                                                                                           |
| `OCI_SSH_KEY`                             |                     | The public SSH key to be added to the Kubernetes nodes. It can be used to login to the node and troubleshoot failures.                                                                                                                                                                                                                                                                                         |
| `OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION` | true                | [In-transit encryption](https://docs.oracle.<br/>com/en-us/iaas/Content/File/Tasks/intransitencryption.htm) provides a way to secure your data between instances and mounted file systems using TLS v.1.2 (Transport Layer Security) encryption. Only [some bare metal instances](https://docs.oracle.com/en-us/iaas/releasenotes/changes/60d602f5-abb3-4639-aa19-292a5744a808/) support In-transit encryption |
| `OCI_WORKER_PV_TRANSIT_ENCRYPTION`        | true                | [In-transit encryption](https://docs.oracle.com/en-us/iaas/Content/File/Tasks/intransitencryption.htm) provides a way to secure your data between instances and mounted file systems using TLS v.1.2 (Transport Layer Security) encryption. Only [some bare metal instances](https://docs.oracle.com/en-us/iaas/releasenotes/changes/60d602f5-abb3-4639-aa19-292a5744a808/) support In-transit encryption      |

The following Cluster API parameters are also available:

| Parameter                      | Default Value          | Description |
| ----------------------------   | ---------------------- | ----------- |
| `CLUSTER_NAME`                 |                        | The name of the workload cluster to create |
| `CONTROL_PLANE_MACHINE_COUNT`  |       1                | The number of control plane machines for the workload cluster.|
| `KUBERNETES_VERSION`           |                        | The Kubernetes version to use for the workload cluster. If unspecified, the value from OS environment variables or the .cluster-api/clusterctl.yaml config file will be used. |
| `NAMESPACE`                    |                        | The namespace to use for the workload cluster. If unspecified, the current namespace will be used |
| `POD_CIDR`                     |       1                | The CIDR range for the Kubernetes POD network. |
| `SERVICE_CIDR`                 |                        | The CIDR for the Kubernetes services network.  |
| `SERVICE_DOMAIN`               |                        |  |
| `WORKER_MACHINE_COUNT`         |                        | The number of worker machines for the workload cluster. |

## Create a new workload cluster on virtual instances using an Ubuntu custom image

Run the command below to create a Kubernetes cluster with 1 control plane node and 1 worker node. This will setup 
both the control plane and the worker nodes using the default information defined in the 
[Workload Cluster Parameters](#workload-cluster-parameters) 
table:

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
CONTROL_PLANE_MACHINE_COUNT=1 \
KUBERNETES_VERSION=v1.20.10 \
NAMESPACE=default \
WORKER_MACHINE_COUNT=1 \
clusterctl generate cluster <cluster-name>\
--from cluster-template.yaml | kubectl apply -f -
```

## Create a new workload cluster on bare metal instances using an Ubuntu custom image

Note the addition of the `OCI_CONTROL_PLANE_SHAPE` variables, `OCI_WORKER_SHAPE` variables to change the shape 
information from the `VM.Standard.E4.Flex` default. You will also need to set`OCI_PV_TRANSIT_ENCRYPTION=false` which is 
required for most BM shapes.

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
OCI_CONTROL_PLANE_SHAPE=BM.Standard2.52 \
OCI_CONTROL_PLANE_SHAPE_OCPUS=52 \
OCI_CONTROL_PLANE_SHAPE_MEMORY_IN_GBS= \
OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION=false \
OCI_WORKER_SHAPE=BM.Standard2.52 \
OCI_WORKER_SHAPE_OCPUS=52 \
OCI_WORKER_SHAPE_MEMORY_IN_GBS= \
OCI_WORKER_PV_TRANSIT_ENCRYPTION=false \
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
[latest-release]: https://github.com/oracle/cluster-api-provider-oci/releases/tag/v0.1.0
[install-oci-ccm]: ./install-oci-ccm.md
