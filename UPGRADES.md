# CAPOCI Upgrade to CAPI v1.11.0 – Complete Changes List

## 1. Dependency Updates
- **go.mod:** `sigs.k8s.io/cluster-api` v1.10.6 → v1.11.0
- **go.mod:** test dependency v1.10.6 → v1.11.0
- **go.mod:** updated replace directive
- Ran `go mod download` and `go mod tidy`

## 2. Import Path Changes (107 files)
- **87 files:**  
  `sigs.k8s.io/cluster-api/api/v1beta1` → `sigs.k8s.io/cluster-api/api/core/v1beta1`
- **20 files:**  
  `sigs.k8s.io/cluster-api/exp/api/v1beta1` → `sigs.k8s.io/cluster-api/api/core/v1beta1`

## 3. Makefile Updates (2 locations)
- Changed  
  `--extra-peer-dirs=sigs.k8s.io/cluster-api/api/v1beta1`  
  →  
  `--extra-peer-dirs=sigs.k8s.io/cluster-api/api/core/v1beta1`

## 4. Conditions Package Migration (18 files)
- Changed  
  `sigs.k8s.io/cluster-api/util/conditions`  
  →  
  `sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions`
- Fixed:
  - 72 uses of `conditions.MarkFalse`
  - 12 uses of `conditions.MarkTrue`
  - 4 uses of `conditions.SetSummary`

## 5. Predicate Updates (4 files)
- Replaced:  
  `ClusterUnpausedAndInfrastructureReady` → `ClusterPausedTransitionsOrInfrastructureProvisioned`
- **Files:**  
  - `ocimachine_controller.go`
  - `ocimachinepool_controller.go`
  - `ocimanaged_machinepool_controller.go`
  - `ocivirtual_machinepool_controller.go`

## 6. ContractVersionedObjectReference Fixes (2 files)
- Changed `if ref nil` → `if ref.Name == ""`
- Changed `ref.Namespace` → `cluster.Namespace`
- **Files:**  
  - `ocimanagedcluster_controller.go`
  - `ocimanagedcluster_controlplane_controller.go`

## 7. v1beta2 to v1beta1 Cluster Conversion (7 files)
- Added `ConvertFrom()` calls after `util.GetOwnerCluster()`
- **Files:**  
  - `ocicluster_controller.go`
  - `ocimachine_controller.go`
  - `ocimanagedcluster_controller.go`
  - `ocimanagedcluster_controlplane_controller.go`
  - `ocimachinepool_controller.go`
  - `ocimanaged_machinepool_controller.go`
  - `ocivirtual_machinepool_controller.go`

## 8. IsControlPlaneMachine Fix (1 file)
- Replaced `capiUtil.IsControlPlaneMachine()` with direct label check using `clusterv1.MachineControlPlaneLabel`
- **File:**  
  - `cloud/scope/machine.go`