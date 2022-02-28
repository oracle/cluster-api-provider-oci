# User configuration and OCIDs

1. Login as the `iaas_oke_usr` in the OCI Console to configure your OCI key. You can either use the OCI console or openssl to [generate an API key][generate_api_key].

1. Obtain the following details which you will need in order to create your management cluster with OKE:
    - `<compartment>` OCID
      - Navigate to Identity > Compartments
      - Click on your compartment
      - Locate OCID on the page and click on **Copy**
    - [Tenancy OCID][ocids]
    - [User OCID][ocids]
    - [API key fingerprint][fingerprint]

[fingerprint]: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#four
[generate_api_key]: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#two
[ocids]: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#five
