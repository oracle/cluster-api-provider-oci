#!/bin/bash
# Copyright (c) 2021, 2022 Oracle and/or its affiliates.

set -o errexit
set -o nounset
set -o pipefail

RUN_TARGET="test-e2e-run"
PREFLIGHT_ONLY="false"

usage() {
    cat <<'EOF'
Usage: scripts/run-e2e.sh [options]

Supported local runner for CAPOCI E2E execution.

Options:
  --auth-config-dir <path>  AUTH_CONFIG_DIR to use for OCI auth.
  --focus <regex>           Override Ginkgo focus.
  --skip <regex>            Override Ginkgo skip.
  --artifacts-dir <path>    Override artifact output directory.
  --ginkgo-nodes <count>    Override Ginkgo parallelism.
  --existing-cluster        Reuse the current management cluster.
  --skip-cleanup            Preserve resources after the run.
  --build-images            Use make test-e2e instead of make test-e2e-run.
  --preflight-only          Run preflight and stop.
  -h, --help                Show this help message.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --auth-config-dir)
            if [ $# -lt 2 ]; then
                echo "missing value for --auth-config-dir" >&2
                exit 1
            fi
            export AUTH_CONFIG_DIR="$2"
            shift 2
            ;;
        --focus)
            export GINKGO_FOCUS="$2"
            shift 2
            ;;
        --skip)
            export GINKGO_SKIP="$2"
            shift 2
            ;;
        --artifacts-dir)
            export ARTIFACTS="$2"
            shift 2
            ;;
        --ginkgo-nodes)
            export GINKGO_NODES="$2"
            shift 2
            ;;
        --existing-cluster)
            export SKIP_CREATE_MGMT_CLUSTER=true
            shift
            ;;
        --skip-cleanup)
            export SKIP_CLEANUP=true
            shift
            ;;
        --build-images)
            RUN_TARGET="test-e2e"
            shift
            ;;
        --preflight-only)
            PREFLIGHT_ONLY="true"
            shift
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

make test-e2e-preflight

if [ "${PREFLIGHT_ONLY}" = "true" ]; then
    exit 0
fi

make "${RUN_TARGET}"
