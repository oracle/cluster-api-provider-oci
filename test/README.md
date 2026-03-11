# CAPOCI E2E Runbook

This is the source-of-truth runbook for CAPOCI E2E execution in this repo.

## Current auth model

The E2E suite currently reads auth from `test/e2e/e2e_suite_test.go`.

- Local instance principal uses `USE_INSTANCE_PRINCIPAL_B64`.
- Local user principal uses base64-encoded env vars such as `OCI_TENANCY_ID_B64`.
- Argo currently supports only instance principal for both suite execution and OCIR login.
- `AUTH_CONFIG_DIR` is not read by the E2E suite today.

## Default suite behavior

- `make test-e2e` builds and pushes the manager image, then runs the E2E suite.
- `make test-e2e-run` skips the image build and push step.
- Default focus is `PRBlocking`.
- Default skip is `Bare Metal|Multi-Region|VCNPeering`.
- Default Ginkgo parallelism is `GINKGO_NODES=3`.

## Required env vars for standard runs

These are required for both local and Argo execution:

```bash
export OCI_COMPARTMENT_ID=<ocid>
export OCI_IMAGE_ID=<ocid>
export OCI_ORACLE_LINUX_IMAGE_ID=<ocid>
export OCI_UPGRADE_IMAGE_ID=<ocid>
export OCI_ALTERNATIVE_REGION_IMAGE_ID=<ocid>
export OCI_MANAGED_NODE_IMAGE_ID=<ocid>
```

Optional standard inputs:

```bash
export OCI_WINDOWS_IMAGE_ID=<ocid>
export OCI_SSH_KEY="ssh-rsa AAAA..."
export TAG=<image-tag>
export GINKGO_NODES=3
export GINKGO_FOCUS="PRBlocking"
export GINKGO_SKIP="Bare Metal|Multi-Region|VCNPeering"
export REGISTRY=<registry-prefix>
```

## Scenario-specific env vars

Windows-focused runs also require:

```bash
export OCI_WINDOWS_IMAGE_ID=<ocid>
```

VCN peering runs also require:

```bash
export LOCAL_DRG_ID=<ocid>
export PEER_DRG_ID=<ocid>
export PEER_REGION_NAME=<region>
export OCI_ALTERNATIVE_REGION=<region>
export EXTERNAL_VCN_ID=<ocid>
export EXTERNAL_VCN_CPE_NSG=<ocid>
export EXTERNAL_VCN_WORKER_NSG=<ocid>
export EXTERNAL_VCN_CP_NSG=<ocid>
export EXTERNAL_VCN_CPE_SUBNET=<ocid>
export EXTERNAL_VCN_WORKER_SUBNET=<ocid>
export EXTERNAL_VCN_CP_SUBNET=<ocid>
```

## Local execution

### 1. Validate prerequisites

Run the preflight before a full suite:

```bash
make test-e2e-preflight
```

The local preflight checks:

- required tools: `go`, `docker`, `kind`, `kubectl`, `kustomize`, `envsubst`, `jq`
- required OCI image inputs
- scenario-specific env vars inferred from `GINKGO_FOCUS`
- whether auth is configured for the current suite

### 2. Configure auth

For local instance principal:

```bash
export USE_INSTANCE_PRINCIPAL_B64="$(printf '%s' "true" | base64 | tr -d '\n')"
```

For local user principal:

```bash
export OCI_TENANCY_ID=<tenancy-ocid>
export OCI_USER_ID=<user-ocid>
export OCI_CREDENTIALS_FINGERPRINT=<fingerprint>
export OCI_REGION=<region>
export OCI_TENANCY_ID_B64="$(printf '%s' "$OCI_TENANCY_ID" | base64 | tr -d '\n')"
export OCI_USER_ID_B64="$(printf '%s' "$OCI_USER_ID" | base64 | tr -d '\n')"
export OCI_CREDENTIALS_FINGERPRINT_B64="$(printf '%s' "$OCI_CREDENTIALS_FINGERPRINT" | base64 | tr -d '\n')"
export OCI_REGION_B64="$(printf '%s' "$OCI_REGION" | base64 | tr -d '\n')"
export OCI_CREDENTIALS_KEY_B64="$(base64 < /path/to/api-private-key.pem | tr -d '\n')"
export OCI_CREDENTIALS_PASSPHRASE_B64="$(printf '%s' "$OCI_CREDENTIALS_PASSPHRASE" | base64 | tr -d '\n')"
```

Notes:

- Session-token envs are not consumed by `test/e2e/e2e_suite_test.go`.
- `AUTH_CONFIG_DIR` does not configure the E2E suite today.

### 3. Run the suite

Full local flow, including image build and push:

```bash
make test-e2e-preflight
REGISTRY=<registry-prefix> scripts/ci-e2e.sh
```

Focused run that reuses the already-built manager image flow:

```bash
make test-e2e-preflight
make test-e2e-run GINKGO_FOCUS="PRBlocking" GINKGO_SKIP="Bare Metal|Multi-Region|VCNPeering"
```

Examples:

```bash
make test-e2e-run GINKGO_SKIP="" GINKGO_FOCUS="VCNPeering"
```

```bash
make test-e2e-run GINKGO_FOCUS="Conformance" E2E_ARGS='-kubetest.config-file=$(pwd)/test/e2e/data/kubetest/conformance.yaml'
```

Artifacts:

- local logs and manifests are written under `_artifacts/`
- rendered config is written to `test/e2e/config/e2e_conf-envsubst.yaml`

## Argo execution

### 1. Validate inputs

Run the Argo-oriented preflight:

```bash
make test-e2e-preflight E2E_PREFLIGHT_ARGS="--mode argo"
```

The Argo preflight checks:

- required tools: `argo`, `kubectl`, `jq`
- workflow/config manifests exist in the repo
- required OCI image inputs are exported
- `REGISTRY` is set for submission
- `USE_INSTANCE_PRINCIPAL=true` is selected

### 2. Prepare the config map

Update `argo/capoci-e2e-configmap.yaml` with the required OCI image IDs and registry, then apply it:

```bash
kubectl apply -f argo/capoci-e2e-configmap.yaml
```

### 3. Submit the workflow

Argo currently supports instance principal only:

```bash
export USE_INSTANCE_PRINCIPAL=true
export REGISTRY=<registry-prefix>
make test-e2e-preflight E2E_PREFLIGHT_ARGS="--mode argo"

argo submit argo/capoci-e2e-workflow.yaml \
  -p git_ref=<branch> \
  -p git_repo=<repo-url> \
  -p registry="${REGISTRY}" \
  -p oci_compartment_id="${OCI_COMPARTMENT_ID}" \
  -p oci_image_id="${OCI_IMAGE_ID}" \
  -p oci_oracle_linux_image_id="${OCI_ORACLE_LINUX_IMAGE_ID}" \
  -p oci_upgrade_image_id="${OCI_UPGRADE_IMAGE_ID}" \
  -p oci_alternative_region_image_id="${OCI_ALTERNATIVE_REGION_IMAGE_ID}" \
  -p oci_managed_node_image_id="${OCI_MANAGED_NODE_IMAGE_ID}" \
  -p use_instance_principal=true
```

You can also install and submit from the workflow template:

```bash
kubectl apply -f argo/capoci-e2e-workflowtemplate.yaml
argo submit --from workflowtemplate/capoci-e2e -n argo -p use_instance_principal=true
```

Argo artifacts:

- `report.json`
- `_artifacts_logs_only`
- `_artifacts`
