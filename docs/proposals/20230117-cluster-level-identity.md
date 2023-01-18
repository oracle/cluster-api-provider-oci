---
title: Cluster Level Identity(aka Multitenancy support)
authors:
- "@shyamradhakrishnan"
- "@joekr"
  reviewers:
  creation-date: 2023-01-18
  last-updated: 2023-01-18
  status: implementable
  see-also:
- https://github.com/oracle/cluster-api-provider-oci/issues/203
  replaces: []
---

## Summary

Cluster API Provider for OCI(CAPOCI) supports a single OCI User Principal currently. This
User Principal is [defined during installation][user-principal] and will be refreshed when the CAPOCI pod is 
restarted.
Some of the customer scenarios requires support for OCI User Principal defined per cluster. This will help
customers create workload clusters using different OCI User Principals such that a single management cluster can 
support a multitenanted architecture.

## Goals

1. To enable OCICluster resources reconciliation use a cluster specific OCI user principal.
2. To maintain backwards compatibility and cause no impact for users who don't intend to make use of
   this capability
## Non-goals

1. This proposal does not solve multitenancy across a single cluster, for example, control plane in  Tenant A worker
  nodes in Tenant B.

## Proposal

### User Stories

#### Story 1 - Use a Cluster specific OCI user principal

A large organization typically consists of many smaller OCI tenancies and compartments. Such a large organization
may run a single Cluster API management cluster in a managed service model by a single team, let us call as Cluster 
Ops team. The Cluster Ops team will have their own api or tools for customers to create and operate workload clusters.
Cluster Ops team will want to make sure that individual teams use OCI user principal specific to their organization to 
create the workload clusters so that the OCI resources are created in the correct tenancy/compartment and individual
team are not able to create clusters in tenancies or compartments in which they do not have access. In order to achieve
this goal, the Cluster Ops team will not use a single user principal to create all workload clusters, instead, the 
individual workload clusters has to be created using OCI user principals tied to individual workload clusters.

### Custom Resource Changes

The following fields will be added to OCIClusterSpec custom resource.

```go
// OCIClusterSpec defines the desired state of OciCluster
type OCIClusterSpec struct {
  // IdentityRef is a reference to an identity(principal) to be used when reconciling this cluster
  // +optional
  IdentityRef *corev1.ObjectReference `json:"identityRef,omitempty"`
}
```
The following custom resources will be added.

```go

type PrincipalType string

const (
    // UserPrincipal represents a user princpal.
    UserPrincipal PrincipalType = "UserPrincipal"
)

// OCIClusterIdentitySpec defines the parameters that are used to create an OCIIdentity.
type OCIClusterIdentitySpec struct {
  // Type is the type of OCI Principal used.
  // UserPrincipal is the only supported value
  Type PrincipalType `json:"type"`
  
  // PrincipalSecret is a secret reference which contains the authentication credentials for the principal.
  // +optional
  PrincipalSecret corev1.SecretReference `json:"clientSecret,omitempty"`

  // AllowedNamespaces is used to identify the namespaces the clusters are allowed to use the identity from.
  // Namespaces can be selected either using an array of namespaces or with label selector.
  // An empty allowedNamespaces object indicates that OCIClusters can use this identity from any namespace.
  // If this object is nil, no namespaces will be allowed (default behaviour, if this field is not provided)
  // A namespace should be either in the NamespaceList or match with Selector to use the identity.
  //
  // +optional
  // +nullable
  AllowedNamespaces *AllowedNamespaces `json:"allowedNamespaces"`
}

// AllowedNamespaces defines the namespaces the clusters are allowed to use the identity from
// NamespaceList takes precedence over the Selector.
type AllowedNamespaces struct {
  // A nil or empty list indicates that OCICluster cannot use the identity from any namespace.
  //
  // +optional
  // +nullable
  NamespaceList []string `json:"list"`
  
  // Selector is a selector of namespaces that OCICluster can
  // use this Identity from. This is a standard Kubernetes LabelSelector,
  // a label query over a set of resources. The result of matchLabels and
  // matchExpressions are ANDed.
  //
  // A nil or empty selector indicates that OCICluster cannot use this
  // OCIClusterIdentity from any namespace.
  // +optional
  Selector *metav1.LabelSelector `json:"selector"`
}

// OCIClusterIdentityStatus defines the observed state of OCIClusterIdentity.
type OCIClusterIdentityStatus struct {
  // Conditions defines current service state of the OCIClusterIdentity.
  // +optional
  Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

type OCIClusterIdentity struct {
  metav1.TypeMeta   `json:",inline"`
  metav1.ObjectMeta `json:"metadata,omitempty"`

  Spec   OCIClusterIdentitySpec   `json:"spec,omitempty"`
  Status OCIClusterIdentityStatus `json:"status,omitempty"`
}

```


### Example Cluster template

```yaml
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCICluster
spec:
  identityRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: OCIClusterIdentity
    name: <test-identity>
    namespace: <test>
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCIClusterIdentity
metadata:
  name: test-identity
  namespace: test
spec:
  type: UserPrincipal
  clientSecret: 
    name: user-credentials
    namespace: secret-namespace
  allowedNamespaces:
    list:
      - test
---
apiVersion: v1
kind: Secret
metadata:
  name: user-credentials
  namespace: secret-namespace
type: Opaque
data:
  tenancy: <>
  user: <>
  passphrase: <> -> optional
  key: <>
  fingerprint: <>
---
```

### Implementation Details

#### Controller changes

Cluster and machine controller will lookup if the cluster has an associated identity. If an identity exists
OCI clients will be created using the corresponding identity principals. If identity does not exist
the old behaviour of using the CAPOCI pod level OCI identity principals will still be used.

#### Webhook changes

OCICluster validation webhook will be enhanced to add the necessary validations such that 
the identityRef kind object is only of supported types(currently only OCIClusterIdentity)

[user-principal]: https://oracle.github.io/cluster-api-provider-oci/gs/install-cluster-api.html#user-principal