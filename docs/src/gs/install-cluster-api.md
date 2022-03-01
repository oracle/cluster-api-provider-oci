
# Install Cluster API Provider for Oracle Cloud Infrastructure

1. If you are not using [kind][kind] for your management cluster, export the `KUBECONFIG` environment variable to point to the correct Kubeconfig file.

   ```shell
   export KUBECONFIG=/path/to/kubeconfig
   ```

2. Create a file `clusterctl.yaml` in `$HOME/.cluster-api/`

   ```shell
     touch "$HOME"/.cluster-api/clusterctl.yaml
   ```

3. Add the Oracle Cloud Infrastructure (OCI) Provider in `clusterctl.yaml`:

   ```yaml
   providers:
     - name: oci
       url: https://github.com/oracle/cluster-api-provider-oci/releases/v0.1.0/infrastructure-components.yaml
       type: InfrastructureProvider
   ```

## Configure authentication

Before installing Cluster API Provider for OCI (CAPOCI), you must first set up your preferred authentication mechanism using specific environment variables:

   ```bash
      export OCI_TENANCY_ID=<tenancy-id>
      export OCI_USER_ID=<user-id>
      export OCI_CREDENTIALS_FINGERPRINT=<fingerprint>
      export OCI_REGION=<region>
      # if Passphrase is present
      export OCI_CREDENTIALS_PASSPHRASE=<passphrase>
      export OCI_TENANCY_ID_B64="$(echo -n "$OCI_TENANCY_ID" | base64 | tr -d '\n')"
      export OCI_CREDENTIALS_FINGERPRINT_B64="$(echo -n "$OCI_CREDENTIALS_FINGERPRINT" | base64 | tr -d '\n')"
      export OCI_USER_ID_B64="$(echo -n "$OCI_USER_ID" | base64 | tr -d '\n')"
      export OCI_REGION_B64="$(echo -n "$OCI_REGION" | base64 | tr -d '\n')"
      export OCI_CREDENTIALS_KEY_B64=$(base64 < <path-to-api-private-key-file> | tr -d '\n')
      # if Passphrase is present
      export OCI_CREDENTIALS_PASSPHRASE_B64="$(echo -n "OCI_CREDENTIALS_PASSPHRASE" | base64 | tr -d '\n')"
   ```

## Initialize management cluster

Initialize management cluster and install CAPOCI

   ```bash
      clusterctl init --infrastructure oci
   ```

## CAPOCI Components

When installing CAPOCI, the following components will be installed in the management cluster:

 1. A custom resource definition (`CRD`) for `OCICluster`, which is a Kubernetes custom resource that represents a workload cluster created in OCI by CAPOCI.
 2. A custom resource definition (`CRD`) for `OCIMachine`, which is a Kubernetes custom resource that represents one node in the workload cluster created in OCI by CAPOCI.
 3. Role-based access control resources for a Kubernetes `Deployment`, `ServiceAccount`, `Role`, `ClusterRole` and `ClusterRoleBinding`
 4. A Kubernetes `Secret` which will hold OCI credentials
 5. A Kubernetes `Deployment` with the CAPOCI image - ghcr.io/oracle/cluster-api-oci-controller: `<version>`

Please inspect the `infrastructure-components.yaml` present in the release artifacts to know more.

[kind]: https://kind.sigs.k8s.io/
