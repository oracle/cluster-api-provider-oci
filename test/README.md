#Prerequisite 
- Install `envsubst`
- Install `kustomize`
- If you have a running `kind` cluster, please delete the `kind` cluster

# Export the following auth settings if user principal is used as credential
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
   export OCI_CREDENTIALS_PASSPHRASE_B64="$(echo -n "$OCI_CREDENTIALS_PASSPHRASE" | base64 | tr -d '\n')"
   ```

# Export the following auth settings if instance principal has to be used
   ```bash
   export USE_INSTANCE_PRINCIPAL_B64="$(echo -n "true" | base64 | tr -d '\n')"
   ```



# Export the following test settings
   ```bash
  export OCI_COMPARTMENT_ID=<>
  export OCI_IMAGE_ID=<>
  export OCI_UPGRADE_IMAGE_ID=<The image to be upgraded to in the upgrade test, image with newer verions of kubernetes)>
  export OCI_ORACLE_LINUX_IMAGE_ID=<The Oracle Linux Image>
   ```
# Execute the test script
   ```bash
   export REGISTRY="iad.ocir.io/<namespace>"
   export LOCAL_ONLY=false
   scripts/ci-e2e.sh
   ```

# For the VCN Peering related tests, the following parameters needs to be set
   ```bash
   export LOCAL_DRG_ID=<The loacl DRG ID which will be used in Local VCN Peering test>
   export OCI_ALTERNATIVE_REGION_IMAGE_ID=<The Custom Image ID in the reegion where workload cluster will be created>
   export OCI_ALTERNATIVE_REGION=<The region where workload cluster will be created in Remote VCN peering test>
   export PEER_REGION_NAME=<The reion where management cluster exists>
   export PEER_DRG_ID=<The DRG ID of the management cluster>
   ```

Then runt he test script by setting the correct focus parameter like below
```bash
GINKGO_SKIP="" GINKGO_FOCUS="VCNPeering" scripts/ci-e2e.sh
```

For the remote VCN peering test to run, the DRG has to be created manually in management cluster VCN,
and routing rules should have been added in management cluster VCN to route the workload cluster traffic to the DRG.
Following are the steps for the same
1. Create a DRG in the region where test needs to run
2. The assumption is that the VCN on which management cluster runs is already created. Add a VCN attachment in the DRG
  as explained in the doc https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/managingDRGs.htm under section "Attaching a VCN to a DRG"
3. Add a routing rule in the VCN Route Rules to forward traffic to workload cluster VCN, in this case, "10.1.0.0/16"
  to the DRG. This is explained in the section "To route a subnet's traffic to a DRG" in the above doc.
