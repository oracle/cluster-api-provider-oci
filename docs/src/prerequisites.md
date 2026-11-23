# Prerequisites

## Requirements

- An active [Oracle Cloud Infrastructure (OCI) tenancy](https://cloud.oracle.com)

- An active user account in the tenancy

- A supported version of [`oci-cli`](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/cliconcepts.htm)

- A supported version of [`kubectl`](https://kubernetes.io/docs/reference/kubectl/kubectl/)

- A [supported version](https://github.com/oracle/cluster-api-provider-oci#support-policy) of [`clusterctl`](https://cluster-api.sigs.k8s.io/clusterctl/overview.html)

## Owner-reference admission permissions

Management clusters using [OwnerReferencesPermissionEnforcement](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#ownerreferencespermissionenforcement)
require the identity writing an owner reference to have `delete` permission on the dependent resource.
When `blockOwnerDeletion` is true, that identity also needs `update` on the owner's `finalizers` subresource.
Keep these permissions when restricting the installed controller roles.

| Writer | Dependent | Owner | `blockOwnerDeletion` | Required permissions |
| --- | --- | --- | --- | --- |
| CAPOCI controller-manager, OKE control-plane reconciliation | CAPI kubeconfig Secret and self-managed node bootstrap Secret | OCIManagedControlPlane | true, via `metav1.NewControllerRef` | `create`/`delete` on core `secrets`; `update` on `infrastructure.cluster.x-k8s.io/ocimanagedcontrolplanes/finalizers` |
| CAPOCI controller-manager, self-managed, managed and virtual pool reconciliation | OCIMachinePoolMachine | OCIMachinePool, OCIManagedMachinePool or OCIVirtualMachinePool | unset | `create`/`delete` on `infrastructure.cluster.x-k8s.io/ocimachinepoolmachines` |

The installed `config/rbac/role.yaml` already grants these permissions. The control-plane finalizer rule
also has a source marker in `controllers/ocimanagedcluster_controlplane_controller.go`.
The Secret references are written in `cloud/scope/managed_control_plane.go`, including through CAPI's
`kubeconfig.GenerateSecretWithOwner`; pool-machine references are written in `cloud/util/util.go`.
The same manager service account is used for all OCI identity modes; OCI IAM policies do not grant Kubernetes RBAC.

CAPI's Cluster, Machine and MachinePool controllers set the owner references on CAPOCI cluster, machine,
control-plane and pool resources. Those writers need `delete` on the CAPOCI dependent resources and,
when they set `blockOwnerDeletion`, `update` on the corresponding CAPI owner's `finalizers` subresource.
These permissions belong to the upstream controller identities. CAPOCI's controllers wait for those references;
they do not write them. See the [InfraCluster](https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-cluster),
[InfraMachine](https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-machine),
[InfraMachinePool](https://cluster-api.sigs.k8s.io/developer/providers/contracts/infra-machinepool) and
[ControlPlane](https://cluster-api.sigs.k8s.io/developer/providers/contracts/control-plane) contracts when customizing roles.
