#Prerequisite 
- Install `envsubst`
- Install `kustomize`
- If you have a running `kind` cluster, please delete the `kind` cluster

# Export the following auth settings
   ```bash
   export OCI_TENANCY_ID=<tenancy-id>
   export OCI_USER_ID=<use-id>
   export OCI_CREDENTIALS_FINGERPRINT=<fingerprint>
   export OCI_REGION=<region>
   # if Passphrase is present
   export OCI_CREDENTIALS_PASSPHRASE=<passphrase>
   export OCI_TENANCY_ID_B64="$(echo -n "$OCI_TENANCY_ID" | base64 | tr -d '\n')"
   export OCI_CREDENTIALS_FINGERPRINT_B64="$(echo -n "$OCI_CREDENTIALS_FINGERPRINT" | base64 | tr -d '\n')"
   export OCI_USER_ID_B64="$(echo -n "$OCI_USER_ID" | base64 | tr -d '\n')"
   export OCI_REGION_B64="$(echo -n "$OCI_REGION" | base64 | tr -d '\n')"
   export OCI_CREDENTIALS_KEY_B64=$( cat <path-to-api-private-key-file> | base64 | tr -d '\n' )
   # if Passphrase is present
   export OCI_CREDENTIALS_PASSPHRASE_B64="$(echo -n "OCI_CREDENTIALS_PASSPHRASE" | base64 | tr -d '\n')"

   ```

# Export the following test settings
   ```bash
  export OCI_COMPARTMENT_ID=<>
  export OCI_IMAGE_ID=<>
  export OCI_UPGRADE_IMAGE_ID=<The image to be upgraded to in the upgrade test, image with newer verions of kubernetes)>
  export OCI_ORACLE_LINUX_IMAGE_ID=<The Oracle Linux Image>
  export OCI_CCM_TEST_IMAGE_ID=<Image which support OCI CSI Driver suupported kubernetes version>
   ```
# Execute the test script
   ```bash
   export REGISTRY="iad.ocir.io/<namespace>"
   export LOCAL_ONLY=false
   scripts/ci-e2e.sh
   ```