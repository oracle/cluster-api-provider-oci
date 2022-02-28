# Install Oracle Cloud Infrastructure Cloud Controller Manager

[Oracle Cloud Infrastructure (OCI) Cloud Controller Manager][oci-ccm] is OCI's implementation of the Kubernetes [control plane component][ccm] that links your Kubernetes cluster to OCI.

## Configure authentication

1. Before downloading the YAML files below, set the version you want to install e.g.

   ```shell
   export CCM_RELEASE_VERSION=0.12.0
   ```

1. Download the example configuration file:

   ```shell
   curl -L https://raw.githubusercontent.com/oracle/oci-cloud-controller-manager/master/manifests/provider-config-example.yaml -o cloud-provider-example.yaml
   ```

1. Update the values as necessary.
1. Create a secret:

   ```shell
   kubectl  create secret generic oci-cloud-controller-manager \
     -n kube-system                                           \
     --from-file=cloud-provider.yaml=cloud-provider-example.yaml
   ```

1. Download the deployment manifests:

  ```shell
  curl -L https://github.com/oracle/oci-cloud-controller-manager/releases/download/${CCM_RELEASE_VERSION}/oci-cloud-controller-manager.yaml -o oci-cloud-controller-manager.yaml

  curl -L https://github.com/oracle/oci-cloud-controller-manager/releases/download/${CCM_RELEASE_VERSION}/oci-cloud-controller-manager-rbac.yaml -o oci-cloud-controller-manager-rbac.yaml
  ```

1. Deploy the CCM:

   ```shell
   kubectl apply -f oci-cloud-controller-manager.yaml
   ```

1. Deploy the RBAC rules:

   ```shell
   kubectl apply -f oci-cloud-controller-manager-rbac.yaml
   ```

1. Check the CCM logs to verify OCI CCM is running correctly:

   ```shell
   kubectl -n kube-system get po | grep oci
   oci-cloud-controller-manager-ds-k2txq   1/1       Running   0          19s

   kubectl -n kube-system logs oci-cloud-controller-manager-ds-k2txq
   ```

[ccm]: https://kubernetes.io/docs/concepts/architecture/cloud-controller/
[oci-ccm]: https://github.com/oracle/oci-cloud-controller-manager
