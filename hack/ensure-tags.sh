#!/bin/bash
# Copyright (c) 2021, 2022 Oracle and/or its affiliates.

set -o errexit
set -o nounset
set -o pipefail

# timestamp is in RFC-3339 format to match kubetest
export TIMESTAMP="${TIMESTAMP:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
export JOB_NAME="${JOB_NAME:-"cluster-api-provider-oci-e2e"}"
if [[ -n "${REPO_OWNER:-}" ]] && [[ -n "${REPO_NAME:-}" ]] && [[ -n "${PULL_BASE_SHA:-}" ]]; then
    export BUILD_PROVENANCE="${REPO_OWNER:-}/${REPO_NAME:-}:${PULL_BASE_SHA:-}"
else
    export BUILD_PROVENANCE="canary"
fi
