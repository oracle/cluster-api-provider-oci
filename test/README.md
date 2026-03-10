# CAPOCI E2E Runbook

This is the source-of-truth runbook for CAPOCI E2E execution in this repo.

## Current auth model

The E2E suite currently reads auth from `test/e2e/e2e_suite_test.go`.

- Preferred local and Argo auth uses `AUTH_CONFIG_DIR` and `cloud/config.FromDir`.
- Legacy local base64 auth envs still work as a compatibility fallback when `AUTH_CONFIG_DIR` is unset.
- Argo currently supports only instance principal for both suite execution and OCIR login.

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
- whether `AUTH_CONFIG_DIR` or the legacy fallback envs are configured for the current suite

### 2. Configure auth

Preferred local auth is `AUTH_CONFIG_DIR`.

For local instance principal:

```bash
mkdir -p /tmp/capoci-auth
printf '%s' true > /tmp/capoci-auth/useInstancePrincipal
export AUTH_CONFIG_DIR=/tmp/capoci-auth
```

For local user principal:

```bash
mkdir -p /tmp/capoci-auth
printf '%s' false > /tmp/capoci-auth/useInstancePrincipal
printf '%s' false > /tmp/capoci-auth/useSessionToken
printf '%s' <region> > /tmp/capoci-auth/region
printf '%s' <tenancy-ocid> > /tmp/capoci-auth/tenancy
printf '%s' <user-ocid> > /tmp/capoci-auth/user
printf '%s' <fingerprint> > /tmp/capoci-auth/fingerprint
cp /path/to/api-private-key.pem /tmp/capoci-auth/key
printf '%s' <passphrase> > /tmp/capoci-auth/passphrase
export AUTH_CONFIG_DIR=/tmp/capoci-auth
```

Notes:

- Legacy base64 envs still work if `AUTH_CONFIG_DIR` is unset.
- Session-token auth also works through `AUTH_CONFIG_DIR` if the directory contains `useSessionToken`, `sessionToken`, and `sessionPrivateKey`.

### 3. Run the suite

Preferred local runner:

```bash
scripts/run-e2e.sh --auth-config-dir "${AUTH_CONFIG_DIR}" --focus "PRBlocking"
```

Full local flow, including image build and push:

```bash
REGISTRY=<registry-prefix> scripts/ci-e2e.sh
```

Examples:

```bash
scripts/run-e2e.sh --auth-config-dir "${AUTH_CONFIG_DIR}" --focus "VCNPeering" --skip ""
```

```bash
scripts/run-e2e.sh --auth-config-dir "${AUTH_CONFIG_DIR}" --existing-cluster --artifacts-dir "$(pwd)/_artifacts-existing"
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

The workflow writes a temporary `AUTH_CONFIG_DIR` inside the job before running `scripts/ci-e2e.sh`.

You can also install and submit from the workflow template:

```bash
kubectl apply -f argo/capoci-e2e-workflowtemplate.yaml
argo submit --from workflowtemplate/capoci-e2e -n argo -p use_instance_principal=true
```

Argo artifacts:

- `report.json`
- `_artifacts_logs_only`
- `_artifacts`
