# Install CSI

On Oracle Cloud Infrastructure (OCI), there are two types of storage services available to store persistent data:

- OCI Block Volume Service
- OCI File Storage Service

A persistent volume claim (PVC) is a request for storage, which is met by binding the PVC to a persistent volume (PV). A PVC provides an abstraction layer to the underlying storage. CSI drivers for both the Block Volume Service and File Storage Service have been implemented.

## Configure authentication via Instance Principal

Oracle recommends using [Instance principals][instance-principals] to be used by CSI for authentication. Please ensure the 
following policies in the dynamic group for CSI to be able to talk to various OCI Services.

```
allow dynamic-group [your dynamic group name] to manage instance-family in compartment [your compartment name]
allow dynamic-group [your dynamic group name] to manage virtual-network-family in compartment [your compartment name]
allow dynamic-group [your dynamic group name] to manage volume-family in compartment [your compartment name]
```

1. Download the example configuration file:

   ```shell
   curl -L https://raw.githubusercontent.com/oracle/oci-cloud-controller-manager/master/manifests/provider-config-instance-principals-example.yaml -o cloud-provider-example.yaml
   ```
2. Update values in the configuration file as necessary.
3. Create a secret:

   ```shell
   kubectl  create secret generic oci-volume-provisioner \
     -n kube-system                                           \
     --from-file=config.yaml=cloud-provider-example.yaml
   ```

### Install CSI Drivers

1. Navigate to the [release page][oci-ccm-release-page] of CCM and export the version that you want to install. Typically, 
   the latest version can be installed.

   ```shell
   export CCM_RELEASE_VERSION=<update-version-here>
   ```

4. Download the deployment manifests:

   ```shell
   curl -L "https://github.com/oracle/oci-cloud-controller-manager/releases/download/${CCM_RELEASE_VERSION}/oci-csi-node-rbac.yaml" -o oci-csi-node-rbac.yaml

   curl -L "https://github.com/oracle/oci-cloud-controller-manager/releases/download/${CCM_RELEASE_VERSION}/oci-csi-controller-driver.yaml" -o oci-csi-controller-driver.yaml

   curl -L h"ttps://github.com/oracle/oci-cloud-controller-manager/releases/download/${CCM_RELEASE_VERSION}/oci-csi-node-driver.yaml" -o
   oci-csi-node-driver.yaml

   curl -L https://raw.githubusercontent.com/oracle/oci-cloud-controller-manager/master/manifests/container-storage-interface/storage-class.yaml -o storage-class.yaml
   ```

5. Create the RBAC rules:

   ```shell
   kubectl apply -f oci-csi-node-rbac.yaml
   ```

6. Deploy the csi-controller-driver. It is provided as a deployment and it has three containers:
    - `csi-provisioner external-provisioner`
    - `csi-attacher external-attacher`
    - `oci-csi-controller-driver`

  ```shell
   kubectl apply -f oci-csi-controller-driver.yaml
   ```

1. Deploy the `node-driver`. It is provided as a daemon set and it has two containers:
    - `node-driver-registrar`
    - `oci-csi-node-driver`

   ```shell
   kubectl apply -f oci-csi-node-driver.yaml
   ```

1. Create the CSI storage class for the Block Volume Service:

   ```shell
   kubectl apply -f storage-class.yaml
   ```

1. Verify the `oci-csi-controller-driver` and `oci-csi-node-controller` are running in your cluster:

   ```shell
   kubectl -n kube-system get po | grep csi-oci-controller
   kubectl -n kube-system get po | grep csi-oci-node
   ```

### Provision PVCs

Follow the guides below to create PVCs based on the service you require:

- [Provision a PVC on the Block Volume Service](pvc-pvc-bv.md)

- [Provision a PVC on the File Storage Service](pvc-fss.md)


[oci-ccm-release-page]: https://github.com/oracle/oci-cloud-controller-manager/releases
[instance-principals]: https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm