#!/usr/bin/env bash
#
# Install Argo Workflows into the Kubernetes cluster selected by an explicit
# kubectl context.
#
# Requirements:
#   - kubectl configured for the target cluster

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." >/dev/null 2>&1 && pwd)"

ARGO_NAMESPACE="argo"
ARGO_VERSION="${ARGO_VERSION:-v3.7.4}"
ARGO_INSTALL_MANIFEST="${ARGO_INSTALL_MANIFEST:-https://github.com/argoproj/argo-workflows/releases/download/${ARGO_VERSION}/quick-start-minimal.yaml}"
WORKFLOW_TEMPLATE_MANIFEST="${REPO_ROOT}/argo/capoci-e2e-workflowtemplate.yaml"
WAIT_TIMEOUT="300s"
CONFIG_MANIFEST=""
KUBE_CONTEXT=""

usage() {
  cat <<EOF
Usage: $0 [options]

Installs Argo Workflows into the cluster selected by the specified kubectl context.
Applies the CAPOCI WorkflowTemplate after Argo is ready. CAPOCI config is
optional and can be supplied with --config-manifest.

Options:
  --context <name>                 kubectl context to target (required)
  --namespace <name>               Namespace to install into (default: ${ARGO_NAMESPACE})
  --argo-version <version>         Argo Workflows version (default: ${ARGO_VERSION})
  --argo-install-manifest <ref>    Argo install manifest URL or local file path
  --config-manifest <path>         Optional CAPOCI config manifest to apply
  --wait-timeout <duration>        Wait timeout for Argo rollout (default: ${WAIT_TIMEOUT})
  -h, --help                       Show this help and exit

Examples:
  $0 --context my-cluster
  $0 --context my-cluster --argo-version v3.7.4
  $0 --context my-cluster --argo-install-manifest ./quick-start-minimal.yaml
  $0 --context my-cluster --config-manifest ./argo/argo-config.yaml
EOF
}

log() {
  printf '[%(%Y-%m-%dT%H:%M:%S%z)T] %s\n' -1 "$*"
}

err() {
  printf '[%(%Y-%m-%dT%H:%M:%S%z)T] ERROR: %s\n' -1 "$*" >&2
}

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    err "Required binary not found in PATH: $1"
    exit 1
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --context)
        KUBE_CONTEXT="${2:-}"
        shift 2
        ;;
      --namespace)
        ARGO_NAMESPACE="${2:-}"
        shift 2
        ;;
      --argo-version)
        ARGO_VERSION="${2:-}"
        ARGO_INSTALL_MANIFEST="https://github.com/argoproj/argo-workflows/releases/download/${ARGO_VERSION}/quick-start-minimal.yaml"
        shift 2
        ;;
      --argo-install-manifest)
        ARGO_INSTALL_MANIFEST="${2:-}"
        shift 2
        ;;
      --config-manifest)
        CONFIG_MANIFEST="${2:-}"
        shift 2
        ;;
      --wait-timeout)
        WAIT_TIMEOUT="${2:-}"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        err "Unknown argument: $1"
        usage
        exit 2
        ;;
    esac
  done
}

validate_args() {
  if [[ -z "${KUBE_CONTEXT}" ]]; then
    err "--context is required"
    exit 2
  fi

  if [[ -z "${ARGO_NAMESPACE}" ]]; then
    err "--namespace must not be empty"
    exit 2
  fi

  if [[ -n "${CONFIG_MANIFEST}" && ! -f "${CONFIG_MANIFEST}" ]]; then
    err "Config manifest not found: ${CONFIG_MANIFEST}"
    exit 2
  fi

  if [[ ! -f "${WORKFLOW_TEMPLATE_MANIFEST}" ]]; then
    err "WorkflowTemplate manifest not found: ${WORKFLOW_TEMPLATE_MANIFEST}"
    exit 1
  fi
}

verify_cluster_access() {
  log "Verifying kubectl access for context '${KUBE_CONTEXT}'."
  kubectl --context "${KUBE_CONTEXT}" cluster-info >/dev/null
}

apply_manifest() {
  log "Applying manifest: ${ARGO_INSTALL_MANIFEST}"
  kubectl --context "${KUBE_CONTEXT}" -n "${ARGO_NAMESPACE}" apply -f "${ARGO_INSTALL_MANIFEST}"
}

ensure_namespace() {
  log "Ensuring namespace '${ARGO_NAMESPACE}' exists."
  kubectl --context "${KUBE_CONTEXT}" create namespace "${ARGO_NAMESPACE}" --dry-run=client -o yaml | kubectl --context "${KUBE_CONTEXT}" apply -f -
}

apply_optional_config() {
  if [[ -z "${CONFIG_MANIFEST}" ]]; then
    log "Skipping CAPOCI config manifest. Pass --config-manifest if you want to apply one."
    return 0
  fi

  log "Applying CAPOCI config manifest: ${CONFIG_MANIFEST}"
  kubectl --context "${KUBE_CONTEXT}" apply -f "${CONFIG_MANIFEST}"
}

apply_workflow_template() {
  log "Applying CAPOCI WorkflowTemplate: ${WORKFLOW_TEMPLATE_MANIFEST}"
  awk -v namespace="${ARGO_NAMESPACE}" '
    BEGIN { replaced=0 }
    !replaced && $0 == "  namespace: argo" {
      print "  namespace: " namespace
      replaced=1
      next
    }
    { print }
  ' "${WORKFLOW_TEMPLATE_MANIFEST}" | kubectl --context "${KUBE_CONTEXT}" apply -f -
}

wait_for_argo() {
  log "Waiting for Argo Workflows namespace '${ARGO_NAMESPACE}'."
  kubectl --context "${KUBE_CONTEXT}" wait --for=create "namespace/${ARGO_NAMESPACE}" --timeout="${WAIT_TIMEOUT}"

  log "Waiting for Argo Workflows CRDs to establish."
  kubectl --context "${KUBE_CONTEXT}" wait --for=condition=Established \
    "crd/clusterworkflowtemplates.argoproj.io" \
    "crd/cronworkflows.argoproj.io" \
    "crd/workflows.argoproj.io" \
    "crd/workflowtaskresults.argoproj.io" \
    "crd/workflowtemplates.argoproj.io" \
    --timeout="${WAIT_TIMEOUT}"

  log "Waiting for Argo Workflows deployments to roll out."
  kubectl --context "${KUBE_CONTEXT}" -n "${ARGO_NAMESPACE}" rollout status deployment/workflow-controller --timeout="${WAIT_TIMEOUT}"
  kubectl --context "${KUBE_CONTEXT}" -n "${ARGO_NAMESPACE}" rollout status deployment/argo-server --timeout="${WAIT_TIMEOUT}"
  kubectl --context "${KUBE_CONTEXT}" -n "${ARGO_NAMESPACE}" rollout status deployment/minio --timeout="${WAIT_TIMEOUT}"
}

main() {
  parse_args "$@"
  require_bin kubectl
  validate_args

  verify_cluster_access

  log "Installing Argo Workflows into context '${KUBE_CONTEXT}' using '${ARGO_INSTALL_MANIFEST}'."
  ensure_namespace
  apply_manifest
  wait_for_argo
  apply_optional_config
  apply_workflow_template

  log "Argo Workflows bootstrap complete for context '${KUBE_CONTEXT}'."
}

main "$@"
