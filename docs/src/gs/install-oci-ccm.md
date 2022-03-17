# Install Oracle Cloud Infrastructure Cloud Controller Manager

[Oracle Cloud Infrastructure (OCI) Cloud Controller Manager][oci-ccm] is OCI's implementation of the Kubernetes [control plane component][ccm] that links your Kubernetes cluster to OCI.

## Configure authentication via Instance Principal (Recommended)

Oracle recommends using [Instance principals][instance-principals] to be used by CCM for authentication. Please ensure the
following policies in the dynamic group for CCM to be able to talk to various OCI Services.

```
allow dynamic-group [your dynamic group name] to read instance-family in compartment [your compartment name]
allow dynamic-group [your dynamic group name] to use virtual-network-family in compartment [your compartment name]
allow dynamic-group [your dynamic group name] to manage load-balancers in compartment [your compartment name]
```

1. Download the example configuration file:

   ```shell
   curl -L https://raw.githubusercontent.com/oracle/oci-cloud-controller-manager/master/manifests/provider-config-instance-principals-example.yaml -o cloud-provider-example.yaml
   ```
2. Update values in the configuration file as necessary.

3. Create a secret:

   ```shell
   kubectl  create secret generic oci-cloud-controller-manager \
     -n kube-system                                           \
     --from-file=cloud-provider.yaml=cloud-provider-example.yaml
   ```

## Install CCM

1. Navigate to the [release page][oci-ccm-release-page] of CCM and export the version that you want to install. Typically,
   the latest version can be installed.

   ```shell
   export CCM_RELEASE_VERSION=<update-version-here>
   ```

2. Download the deployment manifests:

   ```shell
   curl -L "https://github.com/oracle/oci-cloud-controller-manager/releases/download/${CCM_RELEASE_VERSION}/oci-cloud-controller-manager.yaml" -o oci-cloud-controller-manager.yaml

   curl -L "https://github.com/oracle/oci-cloud-controller-manager/releases/download/${CCM_RELEASE_VERSION}/oci-cloud-controller-manager-rbac.yaml" -o oci-cloud-controller-manager-rbac.yaml
   ```

3. Deploy the CCM:

   ```shell
   kubectl apply -f oci-cloud-controller-manager.yaml
   ```

4. Deploy the RBAC rules:

   ```shell
   kubectl apply -f oci-cloud-controller-manager-rbac.yaml
   ```

5. Check the CCM logs to verify OCI CCM is running correctly:

   ```shell
   kubectl -n kube-system get po | grep oci
   oci-cloud-controller-manager-ds-k2txq   1/1       Running   0          19s

   kubectl -n kube-system logs oci-cloud-controller-manager-ds-k2txq
   ```

[ccm]: https://kubernetes.io/docs/concepts/architecture/cloud-controller/
[oci-ccm]: https://github.com/oracle/oci-cloud-controller-manager
[oci-ccm-release-page]: https://github.com/oracle/oci-cloud-controller-manager/releases
