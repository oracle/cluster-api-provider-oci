# Provision a PVC on the Block Volume Service

The Oracle Cloud Infrastructure Block Volume service (the Block Volume service) provides persistent, durable, and high-performance block storage for your data.

## Create a block volume dynamically using a new PVC

If the cluster administrator has not created any suitable PVs that match the PVC request, you can dynamically provision a block volume using the CSI plugin specified by the oci-bv storage class's definition (provisioner: blockvolume.csi.oraclecloud.com).

1. Define a PVC in a yaml file (`csi-bvs-pvc.yaml`)as below:

   ```yaml
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
   name: mynginxclaim
   spec:
    storageClassName: "oci-bv"
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 50Gi
    ```

1. Apply the manifest to create the PVC

   ```shell
   kubectl create -f  csi-bvs-pvc.yaml
   ```

1. Verify that the PVC has been created:

   ```shell
   kubectl get pvc
   ```

1. The output from the above command shows the current status of the PVC:

   ```shell
   NAME               STATUS   VOLUME   CAPACITY   ACCESSMODES   STORAGECLASS   AGE
   mynginxclaim       Pending                                    oci-bv         4m
   ```

   The PVC has a status of Pending because the `oci-bv` storage class's definition includes `volumeBindingMode: WaitForFirstConsumer`.

1. You can use this PVC when creating other objects, such as pods. For example, you could create a new pod from the following pod definition, which instructs the system to use the `mynginxclaim` PVC as the NGINX volume, which is mounted by the pod at `/data`.

   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
    name: nginx
   spec:
    containers:
      - name: nginx
        image: nginx:latest
        ports:
          - name: http
            containerPort: 80
        volumeMounts:
          - name: data
            mountPath: /usr/share/nginx/html
    volumes:
      - name: data
        persistentVolumeClaim:
        claimName: mynginxclaim
    ```

1. After creating the new pod, verify that the PVC has been bound to a new persistent volume (PV):

   ```shell
   kubectl get pvc
   ```

   The output from the above command confirms that the PVC has been bound:

   ```shell
   NAME               STATUS    VOLUME                               CAPACITY   ACCESSMODES   STORAGECLASS   AGE
   mynginxclaim       Bound     ocid1.volume.oc1.iad.<unique_ID>     50Gi       RWO           oci-bv         4m
   ```

1. Verify that the pod is using the new PVC:

   ```shell
   kubectl describe pod nginx
   ```
