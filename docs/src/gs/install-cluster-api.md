
# Install Cluster API Provider for Oracle Cloud Infrastructure

If you are not using [kind][kind] for your management cluster, export the `KUBECONFIG` environment variable to point to the correct Kubeconfig file.

   ```shell
      export KUBECONFIG=/path/to/kubeconfig
   ```

## Configure authentication
Before installing Cluster API Provider for OCI (CAPOCI), you must first set up your preferred
authentication mechanism using specific environment variables.

### User Principal
If the management cluster is hosted outside OCI, for example a Kind cluster, please configure
user principal using the following parameters. Please refer to the [doc][api-signing-key] to generate the required
credentials.

   ```bash
      export OCI_TENANCY_ID=<insert-tenancy-id-here>
      export OCI_USER_ID=<insert-user-ocid-here>
      export OCI_CREDENTIALS_FINGERPRINT=<insert-fingerprint-here>
      export OCI_REGION=<insert-region-here>
      export OCI_TENANCY_ID_B64="$(echo -n "$OCI_TENANCY_ID" | base64 | tr -d '\n')"
      export OCI_CREDENTIALS_FINGERPRINT_B64="$(echo -n "$OCI_CREDENTIALS_FINGERPRINT" | base64 | tr -d '\n')"
      export OCI_USER_ID_B64="$(echo -n "$OCI_USER_ID" | base64 | tr -d '\n')"
      export OCI_REGION_B64="$(echo -n "$OCI_REGION" | base64 | tr -d '\n')"
      export OCI_CREDENTIALS_KEY_B64=$(base64 < <insert-path-to-api-private-key-file-here> | tr -d '\n')
      # if Passphrase is present
      export OCI_CREDENTIALS_PASSPHRASE=<insert-passphrase-here>
      export OCI_CREDENTIALS_PASSPHRASE_B64="$(echo -n "$OCI_CREDENTIALS_PASSPHRASE" | base64 | tr -d '\n')"
   ```

### Instance Principal

If the management cluster is hosted in Oracle Cloud Infrastructure, [Instance principals][instance-principals] authentication
is recommended. Export the following parameters to use Instance Principals. If Instance Principals are used, the user principal
parameters explained in above section will not be used.

   ```bash
      export USE_INSTANCE_PRINCIPAL="true"
      export USE_INSTANCE_PRINCIPAL_B64="$(echo -n "$USE_INSTANCE_PRINCIPAL" | base64 | tr -d '\n')"
   ```
Please ensure the following policies in the dynamic group for CAPOCI to be able to talk to various OCI Services.

```
allow dynamic-group [your dynamic group name] to manage instance-family in compartment [your compartment name]
allow dynamic-group [your dynamic group name] to manage virtual-network-family in compartment [your compartment name]
allow dynamic-group [your dynamic group name] to manage load-balancers in compartment [your compartment name]
```

## Initialize management cluster

Initialize management cluster and install CAPOCI.

The following command will use the [latest version][capoci-latest-release]:

```bash
  clusterctl init --infrastructure oci
```

In production, it is recommended to set a specific released version. 

```bash
  clusterctl init --infrastructure oci:vX.X.X
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
[api-signing-key]: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm
[instance-principals]: https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/callingservicesfrominstances.htm
[capoci-latest-release]: https://github.com/oracle/cluster-api-provider-oci/releases/latest
