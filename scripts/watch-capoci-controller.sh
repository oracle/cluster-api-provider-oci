#!/usr/bin/env bash
#
# Watch for capoci-controller-manager pod and follow its logs.
#
# Features:
# 1) Attempts to connect to a local kind cluster (kind export kubeconfig --name <name>) with retries.
# 2) Waits for the capoci-controller-manager-xxxx pod in the namespace cluster-api-provider-oci-system to be Ready.
# 3) Follows the pod logs and also writes them to a file in the specified output directory (defaults to this script's directory).
#
# Usage:
#   scripts/watch-capoci-controller.sh [--name <cluster-name>] [--output-dir <dir>] [--retries N] [--delay SECONDS]
#
# Examples:
#   scripts/watch-capoci-controller.sh
#   scripts/watch-capoci-controller.sh -n capoci-e2e -o ./_artifacts -r 90 -d 5
#
# Requirements:
#   - kind
#   - kubectl

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-capoci-e2e}"
OUTPUT_DIR="${OUTPUT_DIR:-$SCRIPT_DIR}"
RETRIES="${RETRIES:-60}"         # total attempts for various waits
DELAY_SECONDS="${DELAY_SECONDS:-5}"
NAMESPACE="cluster-api-provider-oci-system"
POD_PREFIX="capoci-controller-manager"

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  -n, --name <cluster-name>     Kind cluster name (default: ${CLUSTER_NAME})
  -o, --output-dir <dir>        Directory for log output (default: ${OUTPUT_DIR})
  -r, --retries <N>             Number of retry attempts (default: ${RETRIES})
  -d, --delay <SECONDS>         Delay between retries in seconds (default: ${DELAY_SECONDS})
  -h, --help                    Show this help and exit

Environment variables also supported:
  CLUSTER_NAME, OUTPUT_DIR, RETRIES, DELAY_SECONDS
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
      -n|--name)
        CLUSTER_NAME="$2"; shift 2 ;;
      -o|--output-dir)
        OUTPUT_DIR="$2"; shift 2 ;;
      -r|--retries)
        RETRIES="$2"; shift 2 ;;
      -d|--delay)
        DELAY_SECONDS="$2"; shift 2 ;;
      -h|--help)
        usage; exit 0 ;;
      --)
        shift; break ;;
      -*)
        err "Unknown option: $1"
        usage
        exit 2 ;;
      *)
        # ignore stray args for now
        shift ;;
    esac
  done
}

ensure_kind_kubeconfig_and_apiserver() {
  local context="kind-${CLUSTER_NAME}"
  local attempt=1

  while (( attempt <= RETRIES )); do
    log "[$attempt/$RETRIES] Exporting kubeconfig for kind cluster '${CLUSTER_NAME}'..."
    # Export kubeconfig; this merges/updates current kubeconfig
    if ! kind export kubeconfig --name "${CLUSTER_NAME}" >/dev/null 2>&1; then
      log "kind export kubeconfig not yet ready, retrying after ${DELAY_SECONDS}s..."
      sleep "${DELAY_SECONDS}"
      ((attempt++))
      continue
    fi

    # Verify the context exists and API is reachable
    if kubectl --context "${context}" get --raw=/healthz >/dev/null 2>&1 || \
       kubectl --context "${context}" get ns >/dev/null 2>&1; then
      log "Kubernetes API for context '${context}' is reachable."
      # Set current context to ensure subsequent kubectl commands target this cluster
      kubectl config use-context "${context}" >/dev/null
      return 0
    fi

    log "API server for '${context}' not yet reachable, retrying after ${DELAY_SECONDS}s..."
    sleep "${DELAY_SECONDS}"
    ((attempt++))
  done

  err "Failed to connect to kind cluster '${CLUSTER_NAME}' after ${RETRIES} attempts."
  exit 1
}

wait_for_namespace() {
  local attempt=1
  while (( attempt <= RETRIES )); do
    if kubectl get ns "${NAMESPACE}" >/dev/null 2>&1; then
      log "Namespace '${NAMESPACE}' is present."
      return 0
    fi
    log "[$attempt/$RETRIES] Waiting for namespace '${NAMESPACE}' to exist... retrying in ${DELAY_SECONDS}s"
    sleep "${DELAY_SECONDS}"
    ((attempt++))
  done

  err "Namespace '${NAMESPACE}' did not appear after ${RETRIES} attempts."
  exit 1
}

find_controller_pod_name() {
  # returns first matching pod name or empty string
  kubectl get pods -n "${NAMESPACE}" --no-headers 2>/dev/null \
    | awk -v pfx="^${POD_PREFIX}-" '$1 ~ pfx {print $1; exit}'
}

wait_for_controller_pod_ready() {
  local attempt=1
  local pod=""

  while (( attempt <= RETRIES )); do
    pod="$(find_controller_pod_name || true)"
    if [[ -n "${pod}" ]]; then
      log "Found pod '${pod}', waiting for Ready condition..."
      if kubectl -n "${NAMESPACE}" wait --for=condition=Ready "pod/${pod}" --timeout=60s >/dev/null 2>&1; then
        log "Pod '${pod}' is Ready."
        echo "${pod}"
        return 0
      fi
      log "Pod '${pod}' not Ready yet, will retry..."
    else
      log "[$attempt/$RETRIES] Controller pod with prefix '${POD_PREFIX}-' not found yet..."
    fi

    sleep "${DELAY_SECONDS}"
    ((attempt++))
  done

  err "Controller pod '${POD_PREFIX}-xxxx' did not become Ready after ${RETRIES} attempts."
  exit 1
}

main() {
  parse_args "$@"
  require_bin kind
  require_bin kubectl

  mkdir -p "${OUTPUT_DIR}"

  log "Using configuration: cluster='${CLUSTER_NAME}', namespace='${NAMESPACE}', output-dir='${OUTPUT_DIR}', retries=${RETRIES}, delay=${DELAY_SECONDS}s"

  ensure_kind_kubeconfig_and_apiserver
  wait_for_namespace

  local pod_name
  pod_name="$(wait_for_controller_pod_ready)"

  local ts
  ts="$(date +%Y%m%d-%H%M%S)"
  local log_file="${OUTPUT_DIR}/${POD_PREFIX}-${CLUSTER_NAME}-${ts}.log"

  log "Following logs for pod '${pod_name}' in namespace '${NAMESPACE}'. Logs will also be written to: ${log_file}"
  log "Press Ctrl+C to stop streaming logs. The file will remain at: ${log_file}"

  # Stream logs, mirror to file.
  # Note: If the pod restarts or gets re-created, this will follow the current pod instance only.
  # If you prefer following the deployment instead, replace with:
  #   kubectl -n "${NAMESPACE}" logs -f deploy/${POD_PREFIX} | tee "${log_file}"
  kubectl -n "${NAMESPACE}" logs -f "${pod_name}" | tee "${log_file}"
}

main "$@"
