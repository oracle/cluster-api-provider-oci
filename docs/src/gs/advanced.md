# Advanced Options

## Disable OCI Client initialization on startup

CAPOCI supports setting OCI principals at [cluster level][cluster-identity], hence CAPOCI can be
installed without providing OCI user credentials. The following environment variable need to be exported
to install CAPOCI without providing any OCI credentials.

   ```shell
   export INIT_OCI_CLIENTS_ON_STARTUP=false
   ```

If the above setting is used, and [Cluster Identity][cluster-identity] is not used, the OCICluster will
go into error state, and the following error will show up in the CAPOCI pod logs as well as when a
`kubectl describe` action is performed on the OCICluster object.


[cluster-identity]: ./multi-tenancy.md