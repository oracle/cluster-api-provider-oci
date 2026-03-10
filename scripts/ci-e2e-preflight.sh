#!/bin/bash
# Copyright (c) 2021, 2022 Oracle and/or its affiliates.

set -o errexit
set -o nounset
set -o pipefail

MODE="local"

usage() {
    cat <<'EOF'
Usage: scripts/ci-e2e-preflight.sh [--mode local|argo]

Validates CAPOCI E2E prerequisites before running a full suite.

Modes:
  local  Validate local scripts/ci-e2e.sh execution inputs.
  argo   Validate Argo submission inputs and instance-principal-only auth.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --mode)
            if [ $# -lt 2 ]; then
                echo "missing value for --mode" >&2
                exit 1
            fi
            MODE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

case "${MODE}" in
    local|argo)
        ;;
    *)
        echo "unsupported mode: ${MODE}" >&2
        usage >&2
        exit 1
        ;;
esac

failures=0

note_ok() {
    echo "OK: $1"
}

note_fail() {
    echo "FAIL: $1" >&2
    failures=$((failures + 1))
}

check_command() {
    local name="$1"
    if command -v "${name}" >/dev/null 2>&1; then
        note_ok "found command ${name}"
    else
        note_fail "missing command ${name}"
    fi
}

check_file() {
    local path="$1"
    if [ -f "${path}" ]; then
        note_ok "found file ${path}"
    else
        note_fail "missing file ${path}"
    fi
}

check_env() {
    local name="$1"
    if [ -n "${!name:-}" ]; then
        note_ok "env ${name} is set"
    else
        note_fail "env ${name} is not set"
    fi
}

decode_base64() {
    local value="$1"
    if decoded=$(printf '%s' "${value}" | base64 --decode 2>/dev/null); then
        printf '%s' "${decoded}"
        return 0
    fi
    if decoded=$(printf '%s' "${value}" | base64 -D 2>/dev/null); then
        printf '%s' "${decoded}"
        return 0
    fi
    return 1
}

check_base64_env() {
    local name="$1"
    local decoded=""
    if [ -z "${!name:-}" ]; then
        note_fail "env ${name} is not set"
        return
    fi
    if decoded=$(decode_base64 "${!name}"); then
        if [ -n "${decoded}" ]; then
            note_ok "env ${name} is set and base64 decodes"
        else
            note_fail "env ${name} decodes to an empty value"
        fi
    else
        note_fail "env ${name} is not valid base64"
    fi
}

uses_instance_principal_local() {
    local decoded=""
    if [ -n "${USE_INSTANCE_PRINCIPAL_B64:-}" ] && decoded=$(decode_base64 "${USE_INSTANCE_PRINCIPAL_B64}"); then
        [ "${decoded}" = "true" ]
        return
    fi
    return 1
}

check_core_envs() {
    check_env OCI_COMPARTMENT_ID
    check_env OCI_IMAGE_ID
    check_env OCI_ORACLE_LINUX_IMAGE_ID
    check_env OCI_UPGRADE_IMAGE_ID
    check_env OCI_ALTERNATIVE_REGION_IMAGE_ID
    check_env OCI_MANAGED_NODE_IMAGE_ID
}

check_focus_specific_envs() {
    local focus="${GINKGO_FOCUS:-PRBlocking}"
    if [[ "${focus}" == *Windows* ]]; then
        check_env OCI_WINDOWS_IMAGE_ID
    fi
    if [[ "${focus}" == *VCNPeering* ]]; then
        check_env LOCAL_DRG_ID
        check_env PEER_DRG_ID
        check_env PEER_REGION_NAME
        check_env OCI_ALTERNATIVE_REGION
        check_env EXTERNAL_VCN_ID
        check_env EXTERNAL_VCN_CPE_NSG
        check_env EXTERNAL_VCN_WORKER_NSG
        check_env EXTERNAL_VCN_CP_NSG
        check_env EXTERNAL_VCN_CPE_SUBNET
        check_env EXTERNAL_VCN_WORKER_SUBNET
        check_env EXTERNAL_VCN_CP_SUBNET
    fi
}

check_local_auth() {
    if [ -n "${AUTH_CONFIG_DIR:-}" ]; then
        echo "WARN: AUTH_CONFIG_DIR is set, but the CAPOCI E2E suite does not read AUTH_CONFIG_DIR today." >&2
    fi

    if uses_instance_principal_local; then
        note_ok "local auth is configured for instance principal via USE_INSTANCE_PRINCIPAL_B64"
        return
    fi

    check_base64_env OCI_TENANCY_ID_B64
    check_base64_env OCI_USER_ID_B64
    check_base64_env OCI_CREDENTIALS_KEY_B64
    check_base64_env OCI_CREDENTIALS_FINGERPRINT_B64
    check_base64_env OCI_REGION_B64
    if [ -n "${OCI_CREDENTIALS_PASSPHRASE_B64:-}" ]; then
        check_base64_env OCI_CREDENTIALS_PASSPHRASE_B64
    fi

    if [ -n "${OCI_SESSION_TOKEN_B64:-}" ] || [ -n "${OCI_SESSION_PRIVATE_KEY_B64:-}" ]; then
        note_fail "session-token auth envs are not consumed by test/e2e/e2e_suite_test.go"
    fi
}

check_argo_auth() {
    if [ "${USE_INSTANCE_PRINCIPAL:-}" = "true" ]; then
        note_ok "argo auth is configured for instance principal"
    else
        note_fail "argo workflow currently supports only USE_INSTANCE_PRINCIPAL=true"
    fi
}

echo "Running CAPOCI E2E preflight in ${MODE} mode"

check_file "scripts/ci-e2e.sh"
check_file "test/e2e/config/e2e_conf.yaml"

if [ "${MODE}" = "local" ]; then
    check_command go
    check_command docker
    check_command kind
    check_command kubectl
    check_command kustomize
    check_command envsubst
    check_command jq
    check_core_envs
    check_focus_specific_envs
    check_local_auth
else
    check_command argo
    check_command kubectl
    check_command jq
    check_file "argo/capoci-e2e-workflow.yaml"
    check_file "argo/capoci-e2e-workflowtemplate.yaml"
    check_file "argo/capoci-e2e-configmap.yaml"
    check_core_envs
    check_focus_specific_envs
    check_env REGISTRY
    check_argo_auth
fi

if [ "${failures}" -ne 0 ]; then
    echo "CAPOCI E2E preflight failed with ${failures} issue(s)." >&2
    exit 1
fi

echo "CAPOCI E2E preflight passed."
