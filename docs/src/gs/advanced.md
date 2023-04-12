# Advanced Options

## Disable OCI Client initialization on startup

CAPOCI supports setting OCI principals at [cluster level][cluster-identity], hence CAPOCI can be
installed without providing OCI user credentials. The following environment variable need to be exported
to install CAPOCI without providing any OCI credentials.

   ```shell
   export INIT_OCI_CLIENTS_ON_STARTUP=false
   ```




[cluster-identity]: ./multi-tenancy.md