

# Create a Windows workload cluster

## Overview

CAPOCI enables users to create and manage Windows workload clusters in Oracle Cloud Infrastructure (OCI).
This means that the [Kubernetes Control Plane][kubernetes-control-plane] will be Linux and the nodes will be Windows.
First, users build the [Windows image using image-builder][image-builder], then use the Windows flavor
template from the [latest release][capoci-latest-release]. Finally, install the [Calico CNI Provider
and OCI Cloud Controller Manager](#install-calico-cni-provider-and-oci-cloud-controller-manager). 

## Known Limitations

The Windows workload cluster has known limitations:

- Limited to [Standard Bare Metal (BM) shapes][bm-shapes]
- Limited to OCI provided platform images. See [image-build documentation][image-builder] for more details
- Custom image MUST be built using the same shape of Bare Metal the worker nodes will run
- CNI provider support is [Calico in VXLAN mode][calico-windows]
- [Block volumes][block-volume] are not currently supported
- Bring Your Own License (BYOL) is not supported
- See [Calico windows docs][calico-limitations] for their limitations 

## Licensing

BYOL is currently not supported using CAPOCI. For more info on Windows Licensing
see the [Compute FAQ documentation][compute-windows-faq].

## Build Windows image

> NOTE: It is recommended to [check shape availability](#check-shape-availability) before building image(s)

In order to launch Windows instances for the cluster a Windows image, [using image-builder][image-builder],
will need to be built. It is **important** to make sure the same shape is used to build and launch the instance.

Example: If a `BM.Standard2.52` is used to build then the `OCI_NODE_MACHINE_TYPE` MUST
be `BM.Standard2.52`


## Check shape availability

Make sure the [OCI CLI][install-oci-cli] is installed. Then set the AD information if using
muti-AD regions.

> NOTE: Use the [OCI Regions and Availability Domains][regions] page to figure out which
regions have multiple ADs.

```bash
oci iam availability-domain list --compartment-id=<your compartment> --region=<region>
```

Using the AD `name` from the output above start searching for BM shape availability.

```bash
oci compute shape list --compartment-id=<your compartment> --profile=DEFAULT --region=us-ashburn-1 --availability-domain=<your AD ID> | grep BM
 
"shape": "BM.Standard.E3.128"
"shape-name": "BM.Standard2.52"
"shape-name": "BM.Standard.E3.128"
"shape": "BM.Standard.E2.64"
"shape-name": "BM.Standard2.52"
"shape-name": "BM.Standard3.64"
"shape": "BM.Standard1.36"
```

> NOTE: If the output is empty then the compartment for that region/AD doesn't have BM shapes.
If you are unable to locate any shapes you may need to submit a
[service limit increase request][compute-service-limit]


## Create a new Windows workload cluster

It is recommended to have the following guides handy:

- [Windows Cluster Debugging][windows-cluster-debug]
- [Windows Container Debugging][windows-containers-debug]

When using `clusterctl` to generate the cluster use the `windows-calico` example flavor. 

The following command uses the `OCI_CONTROL_PLANE_MACHINE_TYPE` and `OCI_NODE_MACHINE_TYPE`
parameters to specify bare metal shapes instead of using CAPOCI's default virtual
instance shape. The `OCI_CONTROL_PLANE_PV_TRANSIT_ENCRYPTION` and `OCI_NODE_PV_TRANSIT_ENCRYPTION`
parameters disable encryption of data in flight between the bare metal instance and the block storage resources.

> NOTE: The `OCI_NODE_MACHINE_TYPE_OCPUS` must match the OPCU count of the BM shape.
See the [Compute Shapes][bm-shapes] page to get the OCPU count for the specific shape.

```bash
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_CONTROL_PLANE_IMAGE_ID=<linux-custom-image-id> \
OCI_NODE_IMAGE_ID=<windows-custom-image-id> \
OCI_SSH_KEY=<ssh-key>  \
NODE_MACHINE_COUNT=1 \
OCI_NODE_MACHINE_TYPE=BM.Standard.E4.128 \
OCI_NODE_MACHINE_TYPE_OCPUS=128 \
OCI_NODE_PV_TRANSIT_ENCRYPTION=false \
OCI_CONTROL_PLANE_MACHINE_TYPE_OCPUS=3 \
OCI_CONTROL_PLANE_MACHINE_TYPE=VM.Standard3.Flex \
CONTROL_PLANE_MACHINE_COUNT=3 \
OCI_SHAPE_MEMORY_IN_GBS= \
KUBERNETES_VERSION=<k8s-version> \
clusterctl generate cluster <cluster-name> \
--target-namespace default \
--flavor windows-calico | kubectl apply -f -
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

### Install Calico CNI Provider and OCI Cloud Controller Manager

#### Install Calico

The [Calico for Windows][calico-windows] getting started guide should be read for better understand of the CNI on Windows.
It is recommended to have the following guides handy:

- [Windows Calico Troubleshooting][calico-windows-debug]

##### The steps to follow:

**On the management cluster**

1. Run 
   ```
   kubectl get OCICluster <cluster-name> -o jsonpath='{.spec.controlPlaneEndpoint.host}'
   ``` 
   to get the `KUBERNETES_SERVICE_HOST` info that will be used in later steps

**On the workload cluster**

1. Download the [v3.24.5 calico release][calico-release] 
   ```
   curl -L https://github.com/projectcalico/calico/releases/download/v3.24.5/release-v3.24.5.tgz -o calico-v3.24.5.tgz
   ```
1. Uncompress the downloaded file and locate the `calico-vxlan.yaml`, `calico-windows-vxlan.yaml` and `windows-kube-proxy.yaml`
files in the `manifests` dir
1. Edit the `calico-vxlan.yaml` and modify the follow variables to allow Calico running on the nodes use VXLAN
   - `CALICO_IPV4POOL_IPIP` - set to `"Never"`
   - `CALICO_IPV4POOL_VXLAN` - set to `"Always"`
1. ```
   kubectl apply -f calico-vxlan.yaml
   ```
1. Wait for the IPAMConfig to be loaded
1. ```
   kubectl patch IPAMConfig default --type merge --patch='{"spec": {"strictAffinity": true}}'
   ```
1. Edit the `calico-windows-vxlan.yaml` and modify the follow variables to allow Calico running on the nodes to talk
to the Kubernetes control plane
   - `KUBERNETES_SERVICE_HOST` - the IP address from the management cluster step
   - `KUBERNETES_SERVICE_PORT`- the port from step the management cluster step
   - `K8S_SERVICE_CIDR` - The service CIDR set in the cluster template
   - `DNS_NAME_SERVERS` - the IP address from dns service
      ```
      kubectl  get svc kube-dns -n kube-system  -o jsonpath='{.spec.clusterIP}'
      ```
   - Change the namespace from `calico-system` to `kube-system`
   - add the following `env` to the container named `node`
     ```yaml
     - name: VXLAN_ADAPTER
       value: "Ethernet 2"
     ```
1. ```
   kubectl apply -f calico-windows-vxlan.yaml
   ``` 
   (it takes a bit for this to pass livenessprobe)
1. Edit the `windows-kube-proxy.yaml` 
    - update the `kube-proxy` container environment variable `K8S_VERSION` to the version of kubernetes you are deploying
    - update the `image` version for the container named `kube-proxy` and make sure to set the
    correct [windows nanoserver version][docker-hub-nanoserver] example: `ltsc2019`
1. ```
   kubectl apply -f windows-kube-proxy.yaml
   ```

#### Install OCI Cloud Controller Manager

By default, the [OCI Cloud Controller Manager (CCM)][oci-ccm] is not installed into a workload cluster. To install the OCI CCM, follow [these instructions][install-oci-ccm].

### Scheduling Windows containers

With the cluster in a ready state and CCM installed, an [example deployment][win-webserver-deployment]
can be used to test that pods are scheduled. Accessing the deployed pods and using `nslookup` you can 
[test that the cluster DNS is working][win-webserver-dns].


[block-volume]: https://docs.oracle.com/en-us/iaas/Content/GSG/Tasks/addingstorage.htm
[bm-shapes]: https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm#bm-standard
[calico-limitations]: https://docs.tigera.io/calico/3.24/getting-started/kubernetes/windows-calico/limitations
[calico-release]: https://github.com/projectcalico/calico/releases/download/v3.24.5/release-v3.24.5.tgz
[calico-windows]: https://docs.tigera.io/calico/3.24/getting-started/kubernetes/windows-calico/
[calico-windows-debug]: https://docs.tigera.io/calico/3.24/getting-started/kubernetes/windows-calico/troubleshoot
[capoci-latest-release]: https://github.com/oracle/cluster-api-provider-oci/releases/latest
[compute-service-limit]: https://docs.oracle.com/en-us/iaas/Content/General/Concepts/servicelimits.htm#computelimits
[compute-windows-faq]: https://www.oracle.com/cloud/compute/faq/#category-windows
[docker-hub-nanoserver]: https://hub.docker.com/_/microsoft-windows-nanoserver
[image-builder]: https://image-builder.sigs.k8s.io/capi/providers/oci.html#building-a-windows-image
[install-ccm]: ../gs/create-workload-cluster.md#install-oci-cloud-controller-manager-and-csi-in-a-self-provisioned-cluster
[install-oci-ccm]: ./install-oci-ccm.md
[install-oci-cli]: https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm
[kubernetes-control-plane]: https://kubernetes.io/docs/concepts/overview/components/#control-plane-components
[oci-ccm]: https://github.com/oracle/oci-cloud-controller-manager
[regions]: https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
[win-webserver-deployment]: https://kubernetes.io/docs/concepts/windows/user-guide/
[win-webserver-dns]: https://kubernetes.io/docs/tutorials/services/connect-applications-service/#dns
[windows-cluster-debug]: https://kubernetes.io/docs/tasks/debug/debug-cluster/windows/
[windows-containers-debug]: https://learn.microsoft.com/en-us/virtualization/windowscontainers/kubernetes/common-problems


