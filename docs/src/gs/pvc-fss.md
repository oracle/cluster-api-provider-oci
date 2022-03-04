# Provision a PVC on the File Storage Service

The Oracle Cloud Infrastructure File Storage service provides a durable, scalable, distributed, enterprise-grade network file system.

## Provision PVCs on the File Storage Service (FSS)

Provisioning PVCs on FSS consists of 3 steps:

1. Create an FSS instance and a mount target
2. Define and create a PV backed by FSS
3. Define and create a PVC provisioned by the PV

### Configure the VCN and create an FSS

1. Choose your [deployment scenario][fss-scenarios]:
   1. Mount target and worker nodes in the same subnet
   2. Mount target and worker nodes in different subnets
   3. Mount target and instance use in-transit encryption
2. Implement [security rules][fss-nsg] based on your deployment scenario
3. [Create a FSS instance][create-fss]

[fss-scenarios]: https://docs.oracle.com/en-us/iaas/Content/File/Tasks/securitylistsfilestorage.htm#File_Storage_Security_Rule_Scenarios
[create-fss]: https://docs.oracle.com/en-us/iaas/Content/File/Tasks/creatingfilesystems.htm#createfs
[fss-nsg]: https://docs.oracle.com/en-us/iaas/Content/File/Tasks/securitylistsfilestorage.htm#Setting2

### Define and create a PV backed by FSS

1. Create a yaml file (`fss-pv.yaml`) to define the PV:

   ```yaml
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: fss-pv
   spec:
    capacity:
      storage: 50Gi
    volumeMode: Filesystem
    accessModes:
      - ReadWriteMany
    persistentVolumeReclaimPolicy: Retain
    csi:
      driver: fss.csi.oraclecloud.com
      volumeHandle: ocid1.filesystem.oc1.iad.aaaa______j2xw:10.0.0.6:/FileSystem1
   ```

1. In the yaml file, set:
   1. `driver` value to: `fss.csi.oraclecloud.com`
   2. `volumeHandle` value to: `<FileSystemOCID>:<MountTargetIP>:<path>`

   The `<FileSystemOCID>` is the OCID of the file system defined in the File Storage service, the `<MountTargetIP>` is the IP address assigned to the mount target and the `<path>` is the mount path to the file system relative to the mount target IP address, starting with a slash.

1. Create the PV from the manifest file:

   ```shell
   kubectl create -f fss-pv.yaml
   ```

### Define and create a PVC provisioned by the PV

1. Create a manifest file (`fss-pvc.yaml`) to define the PVC:

   ```yaml
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: fss-pvc
   spec:
     accessModes:
       - ReadWriteMany
     storageClassName: ""
     resources:
       requests:
         storage: 50Gi
     volumeName: fss-pv
   ```

1. In the YAML file, set:
   1. `storageClassName` to `""`
   2. `volumeName` to the name of the PV created earlier

1. Create the PVC from the manifest file:

   ```shell
   kubectl create -f fss-pvc.yaml
   ```
