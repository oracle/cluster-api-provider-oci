## OpenID Connect (OIDC) with Cluster API Provider for Oracle Cloud Infrastructure (CAPOCI)

### Overview

Cluster API Provider for Oracle Cloud Infrastructure (CAPOCI) allows you to manage Kubernetes clusters on Oracle Cloud Infrastructure (OCI). Enabling OIDC in managed clusters using CAPOCI involves configuring the cluster to use OIDC for authentication and ensuring that the necessary components are set up correctly.

### Prerequisites

1. **OIDC Provider**: You need an OIDC provider (e.g., Auth0, Okta, Google Identity Platform, Oracle IDCS,etc.).
2. Ability to create Enhanced OKE clusters.

#### Update CAPOCI Configuration

The example below shows how to update the CAPOCI configuration to include the OIDC settings. This involves modifying the `OCIManagedControlPlane` resource to enable OIDC authentication.

**Example `OCIManagedControlPlane` Configuration:**

```
kind: OCIManagedControlPlane
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
metadata:
  name: "${CLUSTER_NAME}"
  namespace: "${NAMESPACE}"
spec:
  version: "${KUBERNETES_VERSION}"
  clusterType: "ENHANCED_CLUSTER"
  clusterOptions:
    openIdConnectDiscovery:
      isOpenIdConnectDiscoveryEnabled: true
    openIdConnectTokenAuthenticationConfig:
      isOpenIdConnectAuthEnabled: true
      clientId: "<OIDC Configuration Client ID>"
      issuerUrl: "<OIDC issuer URL>"
      groupsClaim: "<OIDC Configuration Groups Claim>"
      groupsPrefix: "<OIDC Configuration Groups Prefix>"
      usernameClaim: "<OIDC Configuration Username Claim>"
      requiredClaims:
        - "<OIDC Configuration Required Claims(key: value)>"
      groupsPrefix: "<OIDC Configuration Groups Prefix>"
      usernamesPrefix: "<OIDC Configuration Usernames Prefix>"
      signingAlgorithm: "<OIDC Configuration Signing Algorithm>"
      caCertificate: "<OIDC Configuration CA Certificate>"
```

**Explanation of Fields:**

- `clusterType`: Specifies the type of cluster. For OIDC, it should be set to `ENHANCED_CLUSTER`. This feature is not available for basic clusters.
- `openIdConnectDiscovery`: Enables OIDC discovery. `isOpenIdConnectDiscoveryEnabled` should be set to `true`.
- `isOpenIdConnectAuthEnabled`: Set to `true` to enable OIDC authentication.
- `clientId`: The client ID obtained from your OIDC provider.
- `issuerUrl`: The issuer URL of your OIDC provider.
- `groupsClaim`: The claim to use for group membership (optional).
- `usernameClaim`: The claim to use for the username (optional).
- `requiredClaims`: Additional claims that must be present in the token (optional).
- `groupsPrefix`: Prefix to add to group names (optional).
- `usernamesPrefix`: Prefix to add to usernames (optional).
- `signingAlgorithm`: The signing algorithm used by the OIDC provider (optional, default is [\"RS256\"]).
- `caCertificate`: The CA certificate used to verify the OIDC provider's TLS certificate (optional).

**Note:** Ensure that the values for `clientId`, `issuerUrl`, and other fields are correctly configured according to your OIDC provider's settings.