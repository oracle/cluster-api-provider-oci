# Create a GPU workload cluster

## Accessing GPU Shapes

Some shapes are limited to specific regions and specific Availability Domains (AD).
In order to make sure the workload cluster comes up check the region and AD for 
shape availability.

### Check shape availability

Make sure the [OCI CLI][install-oci-cli] is installed. Then set the AD information if using 
muti-AD regions.

> NOTE: Use the [OCI Regions and Availability Domains][regions] page to figure out which 
regions have multiple ADs.

```bash
oci iam availability-domain list --compartment-id=<your compartment> --region=<region>
```

Using the AD `name` from the output start searching for GPU shape availability.

```bash
oci compute shape list --compartment-id=<your compartment> --profile=DEFAULT --region=us-ashburn-1 --availability-domain=<your AD ID> | grep GPU
 
"shape-name": "BM.GPU3.8"
"shape-name": "BM.GPU4.8"
"shape-name": "VM.GPU3.1"
"shape": "VM.GPU2.1"
```

> NOTE: If the output is empty then the compartment for that region/AD doesn't have GPU shapes.
If you are unable to locate any shapes you may need to submit a
[service limit increase request][compute-service-limit]

## Create a new GPU workload cluster an Ubuntu custom image

> NOTE: Nvidia GPU drivers aren't supported for Oracle Linux at this time. Ubuntu is currently
the only supported OS.

When launching a multi-AD region shapes are likely be limited to a specific AD (example: `US-ASHBURN-AD-2`).
To make sure the cluster comes up without issue specifically target just that AD for the GPU worker nodes.
To do that modify the released version of the `cluster-template-failure-domain-spread.yaml` template.

Download the [latest `cluster-template-failure-domain-spread.yaml`][releases] file and save it as
`cluster-template-gpu.yaml`.

Make sure the modified template has only the `MachineDeployment` section(s) where there is GPU
availability and remove all the others. See [the full example file][example-yaml-file]
that targets only AD 2 (OCI calls them Availability Domains while Cluster-API calls them Failure Domains).

### Virtual instances 

The following command will create a workload cluster comprising a single 
control plane node and single GPU worker node using the default values as specified in the preceding 
[Workload Cluster Parameters][workload-cluster-parameters] table:

> NOTE: The `OCI_NODE_MACHINE_TYPE_OCPUS` must match the OPCU count of the GPU shape.
See the [Compute Shapes][compute-shapes] page to get the OCPU count for the specific shape.

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
NODE_MACHINE_COUNT=1 \
OCI_NODE_MACHINE_TYPE=VM.GPU3.1 \
OCI_NODE_MACHINE_TYPE_OCPUS=6 \
OCI_CONTROL_PLANE_MACHINE_TYPE_OCPUS=1 \
OCI_CONTROL_PLANE_MACHINE_TYPE=VM.Standard3.Flex \
CONTROL_PLANE_MACHINE_COUNT=1 \
OCI_SHAPE_MEMORY_IN_GBS= \
KUBERNETES_VERSION=v1.24.4 \
clusterctl generate cluster <cluster-name> \
--target-namespace default \
--from cluster-template-gpu.yaml | kubectl apply -f -
```

### Bare metal instances

The following command uses the `OCI_CONTROL_PLANE_MACHINE_TYPE` and `OCI_NODE_MACHINE_TYPE` 
parameters to specify bare metal shapes instead of using CAPOCI's default virtual 
instance shape. The `OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION` and `OCI_NODE_PV_TRANSIT_ENCRYPTION` 
parameters disable encryption of data in flight between the bare metal instance and the block storage resources.

> NOTE: The `OCI_NODE_MACHINE_TYPE_OCPUS` must match the OPCU count of the GPU shape.
See the [Compute Shapes][compute-shapes] page to get the OCPU count for the specific shape.

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
OCI_NODE_MACHINE_TYPE=BM.GPU3.8 \
OCI_NODE_MACHINE_TYPE_OCPUS=52 \
OCI_NODE_PV_TRANSIT_ENCRYPTION=false \
OCI_CONTROL_PLANE_MACHINE_TYPE=VM.Standard3.Flex \
CONTROL_PLANE_MACHINE_COUNT=1 \
OCI_SHAPE_MEMORY_IN_GBS= \
KUBERNETES_VERSION=v1.24.4 \
clusterctl generate cluster <cluster-name> \
--target-namespace default \
--from cluster-template-gpu.yaml | kubectl apply -f -
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

### Install a CNI Provider, OCI Cloud Controller Manager and CSI in a self-provisioned cluster

To provision the CNI and Cloud Controller Manager follow the [Install a CNI Provider][install-a-cni-provider] 
and the [Install OCI Cloud Controller Manager][install-ccm] sections.

### Install Nvidia GPU Operator

Setup the worker instances to use the GPUs install the [Nvidia GPU Operator][nvidia-overview]. 

For the most up-to-date install instructions see the [official install instructions][nvidia-install-gpu-operator]. They
layout how to install the [Helm tool][helm-install] and how to setup the Nvidia helm repo. 

With Helm setup you can now install the GPU-Operator

```bash
helm install --wait --generate-name \
     -n gpu-operator --create-namespace \
     nvidia/gpu-operator
```

The pods will take a while to come up but you can check the status:

```bash
kubectl --<cluster-name>.kubeconf get pods -n gpu-operator
```

### Test GPU on worker node

Once all of the GPU-Operator pods are `running` or `completed` deploy the test pod:

```bash
cat <<EOF | kubectl --kubeconfig=<cluster-name>.kubeconf apply -f -
apiVersion: v1
kind: Pod
metadata:
name: cuda-vector-add
spec:
restartPolicy: OnFailure
containers:
- name: cuda-vector-add
# https://github.com/kubernetes/kubernetes/blob/v1.7.11/test/images/nvidia-cuda/Dockerfile
image: "registry.k8s.io/cuda-vector-add:v0.1"
resources:
limits:
nvidia.com/gpu: 1 # requesting 1 GPU
EOF
```

Then check the output logs of the `cuda-vector-add` test pod:

```bash
kubectl --kubeconfig=<cluster-name>.kubeconf logs cuda-vector-add -n default
 
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## Example yaml file

This is an example file using a modified version of `cluster-template-failure-domain-spread.yaml` 
to target AD 2 (example: `US-ASHBURN-AD-2`). 

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: "${CLUSTER_NAME}"
  name: "${CLUSTER_NAME}"
  namespace: "${NAMESPACE}"
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
        - ${POD_CIDR:="192.168.0.0/16"}
    serviceDomain: ${SERVICE_DOMAIN:="cluster.local"}
    services:
      cidrBlocks:
        - ${SERVICE_CIDR:="10.128.0.0/12"}
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: OCICluster
    name: "${CLUSTER_NAME}"
    namespace: "${NAMESPACE}"
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane
    name: "${CLUSTER_NAME}-control-plane"
    namespace: "${NAMESPACE}"
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCICluster
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: "${CLUSTER_NAME}"
  name: "${CLUSTER_NAME}"
spec:
  compartmentId: "${OCI_COMPARTMENT_ID}"
---
kind: KubeadmControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
  namespace: "${NAMESPACE}"
spec:
  version: "${KUBERNETES_VERSION}"
  replicas: ${CONTROL_PLANE_MACHINE_COUNT}
  machineTemplate:
    infrastructureRef:
      kind: OCIMachineTemplate
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      name: "${CLUSTER_NAME}-control-plane"
      namespace: "${NAMESPACE}"
  kubeadmConfigSpec:
    clusterConfiguration:
      kubernetesVersion: ${KUBERNETES_VERSION}
      apiServer:
        certSANs: [localhost, 127.0.0.1]
      dns: {}
      etcd: {}
      networking: {}
      scheduler: {}
    initConfiguration:
      nodeRegistration:
        criSocket: /var/run/containerd/containerd.sock
        kubeletExtraArgs:
          cloud-provider: external
          provider-id: oci://{{ ds["id"] }}
    joinConfiguration:
      discovery: {}
      nodeRegistration:
        criSocket: /var/run/containerd/containerd.sock
        kubeletExtraArgs:
          cloud-provider: external
          provider-id: oci://{{ ds["id"] }}
---
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
metadata:
  name: "${CLUSTER_NAME}-control-plane"
spec:
  template:
    spec:
      imageId: "${OCI_IMAGE_ID}"
      compartmentId: "${OCI_COMPARTMENT_ID}"
      shape: "${OCI_CONTROL_PLANE_MACHINE_TYPE=VM.Standard.E4.Flex}"
      shapeConfig:
        ocpus: "${OCI_CONTROL_PLANE_MACHINE_TYPE_OCPUS=1}"
      metadata:
        ssh_authorized_keys: "${OCI_SSH_KEY}"
      isPvEncryptionInTransitEnabled: ${OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION=true}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCIMachineTemplate
metadata:
  name: "${CLUSTER_NAME}-md"
spec:
  template:
    spec:
      imageId: "${OCI_IMAGE_ID}"
      compartmentId: "${OCI_COMPARTMENT_ID}"
      shape: "${OCI_NODE_MACHINE_TYPE=VM.Standard.E4.Flex}"
      shapeConfig:
        ocpus: "${OCI_NODE_MACHINE_TYPE_OCPUS=1}"
      metadata:
        ssh_authorized_keys: "${OCI_SSH_KEY}"
      isPvEncryptionInTransitEnabled: ${OCI_NODE_PV_TRANSIT_ENCRYPTION=true}
---
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha4
kind: KubeadmConfigTemplate
metadata:
  name: "${CLUSTER_NAME}-md"
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          kubeletExtraArgs:
            cloud-provider: external
            provider-id: oci://{{ ds["id"] }}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: "${CLUSTER_NAME}-fd-2-md-0"
spec:
  clusterName: "${CLUSTER_NAME}"
  replicas: ${NODE_MACHINE_COUNT}
  selector:
    matchLabels:
  template:
    spec:
      clusterName: "${CLUSTER_NAME}"
      version: "${KUBERNETES_VERSION}"
      bootstrap:
        configRef:
          name: "${CLUSTER_NAME}-md"
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
      infrastructureRef:
        name: "${CLUSTER_NAME}-md"
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: OCIMachineTemplate
      # Cluster-API calls them Failure Domains while OCI calls them Availability Domains
      # In the example this would be targeting US-ASHBURN-AD-2
      failureDomain: "2"
```

[workload-cluster-parameters]: ../gs/create-workload-cluster.md#workload-cluster-parameters
[install-a-cni-provider]: ../gs/create-workload-cluster.md#install-a-cni-provider
[install-ccm]: ../gs/create-workload-cluster.md#install-oci-cloud-controller-manager-and-csi-in-a-self-provisioned-cluster
[install-oci-cli]: https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm
[regions]: https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
[compute-service-limit]: https://docs.oracle.com/en-us/iaas/Content/General/Concepts/servicelimits.htm#computelimits
[example-yaml-file]: #example-yaml-file
[nvidia-overview]: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/overview.html
[nvidia-install-gpu-operator]: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/getting-started.html#install-nvidia-gpu-operator
[helm-install]: https://helm.sh/docs/intro/install/
[compute-shapes]: https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm
[releases]: https://github.com/oracle/cluster-api-provider-oci/releases