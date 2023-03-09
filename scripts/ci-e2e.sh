#!/bin/bash
# Copyright (c) 2021, 2022 Oracle and/or its affiliates.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# shellcheck source=hack/ensure-go.sh
source "${REPO_ROOT}/hack/ensure-go.sh"
# shellcheck source=hack/ensure-kind.sh
source "${REPO_ROOT}/hack/ensure-kind.sh"
# shellcheck source=hack/ensure-kubectl.sh
source "${REPO_ROOT}/hack/ensure-kubectl.sh"
# shellcheck source=hack/ensure-tags.sh
source "${REPO_ROOT}/hack/ensure-tags.sh"

# Verify the required Environment Variables are present.
: "${OCI_COMPARTMENT_ID:?Environment variable empty or not defined.}"
: "${OCI_IMAGE_ID:?Environment variable empty or not defined.}"
: "${OCI_ORACLE_LINUX_IMAGE_ID:?Environment variable empty or not defined.}"
: "${OCI_UPGRADE_IMAGE_ID:?Environment variable empty or not defined.}"
: "${OCI_ALTERNATIVE_REGION_IMAGE_ID:?Environment variable empty or not defined.}"
: OCI_MANAGED_NODE_IMAGE_ID
: OCI_WINDOWS_IMAGE_ID

export LOCAL_ONLY=${LOCAL_ONLY:-"true"}

defaultTag=$(date -u '+%Y%m%d%H%M%S')
export TAG="${defaultTag:-dev}"
export GINKGO_NODES=3

export OCI_SSH_KEY="${OCI_SSH_KEY:-""}"
export OCI_CONTROL_PLANE_MACHINE_TYPE="${OCI_CONTROL_PLANE_MACHINE_TYPE:-"VM.Standard.E3.Flex"}"
export OCI_CONTROL_PLANE_MACHINE_TYPE_OCPUS="${OCI_CONTROL_PLANE_MACHINE_TYPE_OCPUS:-"1"}"
export OCI_NODE_MACHINE_TYPE="${OCI_NODE_MACHINE_TYPE:-"VM.Standard.E3.Flex"}"
export OCI_NODE_MACHINE_TYPE_OCPUS="${OCI_NODE_MACHINE_TYPE_OCPUS:-"1"}"
export KIND_EXPERIMENTAL_DOCKER_NETWORK="bridge"
export OCI_MANAGED_NODE_SHAPE="${OCI_NODE_MACHINE_TYPE:-"VM.Standard.E4.Flex"}"
export OCI_ALTERNATIVE_REGION="${OCI_ALTERNATIVE_REGION:-"us-sanjose-1"}"

# Generate SSH key.
if [ -z "${OCI_SSH_KEY}" ]; then
    echo "generating sshkey for e2e"
    SSH_KEY_FILE=.sshkey
    rm -f "${SSH_KEY_FILE}" 2>/dev/null
    ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N '' 1>/dev/null
    OCI_SSH_KEY=$(cat "${SSH_KEY_FILE}.pub")
    export OCI_SSH_KEY
fi

make test-e2e
