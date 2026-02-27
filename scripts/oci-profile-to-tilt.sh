#!/usr/bin/env bash

set -euo pipefail

PROFILE="DEFAULT"
FORMAT="json"
CONFIG_FILE="${HOME}/.oci/config"
UPDATE_FILE=""

usage() {
  cat <<'EOF'
Usage: scripts/oci-profile-to-tilt.sh [--profile PROFILE] [--format json|env] [--update FILE]

Reads OCI profile values from ~/.oci/config and prints session-token auth values
for Tilt. With --update, it patches an existing tilt-settings.json in-place and
preserves non-auth keys (such as extra_args).

Examples:
  scripts/oci-profile-to-tilt.sh
  scripts/oci-profile-to-tilt.sh --profile DEFAULT --format env
  scripts/oci-profile-to-tilt.sh --profile DEFAULT --update tilt-settings.json
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile)
      PROFILE="${2:-}"
      shift 2
      ;;
    --format)
      FORMAT="${2:-}"
      shift 2
      ;;
    --update)
      UPDATE_FILE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ ! -f "${CONFIG_FILE}" ]]; then
  echo "OCI config not found: ${CONFIG_FILE}" >&2
  exit 1
fi

profile_value() {
  local profile="$1"
  local key="$2"
  awk -v profile="${profile}" -v key="${key}" '
    $0 == "[" profile "]" { in_profile=1; next }
    /^\[/ { in_profile=0 }
    in_profile && $0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
      line=$0
      sub("^[[:space:]]*" key "[[:space:]]*=[[:space:]]*", "", line)
      print line
      exit
    }
  ' "${CONFIG_FILE}"
}

expand_path() {
  local path="$1"
  path="${path/#\~/${HOME}}"
  path="${path//\$\{HOME\}/${HOME}}"
  path="${path//\$HOME/${HOME}}"
  printf '%s' "${path}"
}

b64() {
  printf '%s' "$1" | base64 | tr -d '\n'
}

tenancy="$(profile_value "${PROFILE}" "tenancy")"
region="$(profile_value "${PROFILE}" "region")"
fingerprint="$(profile_value "${PROFILE}" "fingerprint")"
token_path_raw="$(profile_value "${PROFILE}" "security_token_file")"
private_key_path_raw="$(profile_value "${PROFILE}" "key_file")"

if [[ -z "${token_path_raw}" ]]; then
  token_path_raw="${HOME}/.oci/sessions/${PROFILE}/token"
fi
if [[ -z "${private_key_path_raw}" ]]; then
  private_key_path_raw="${HOME}/.oci/sessions/${PROFILE}/oci_api_key.pem"
fi

token_path="$(expand_path "${token_path_raw}")"
private_key_path="$(expand_path "${private_key_path_raw}")"

if [[ -z "${tenancy}" || -z "${region}" || -z "${fingerprint}" ]]; then
  echo "Missing one or more required profile fields (tenancy, region, fingerprint) in [${PROFILE}]" >&2
  exit 1
fi

if [[ ! -f "${token_path}" ]]; then
  echo "Token file not found: ${token_path}" >&2
  exit 1
fi
if [[ ! -f "${private_key_path}" ]]; then
  echo "Private key file not found: ${private_key_path}" >&2
  exit 1
fi

session_token_b64="$(base64 < "${token_path}" | tr -d '\n')"
session_key_b64="$(base64 < "${private_key_path}" | tr -d '\n')"

if [[ "${FORMAT}" == "env" ]]; then
  cat <<EOF
OCI_TENANCY_ID=${tenancy}
OCI_REGION=${region}
OCI_CREDENTIALS_FINGERPRINT=${fingerprint}
OCI_SESSION_TOKEN_PATH=${token_path}
OCI_SESSION_PRIVATE_KEY_PATH=${private_key_path}
USE_INSTANCE_PRINCIPAL_B64=$(b64 "false")
USE_SESSION_TOKEN_B64=$(b64 "true")
OCI_TENANCY_ID_B64=$(b64 "${tenancy}")
OCI_REGION_B64=$(b64 "${region}")
OCI_CREDENTIALS_FINGERPRINT_B64=$(b64 "${fingerprint}")
OCI_SESSION_TOKEN_B64=${session_token_b64}
OCI_SESSION_PRIVATE_KEY_B64=${session_key_b64}
EOF
  exit 0
fi

if [[ "${FORMAT}" != "json" ]]; then
  echo "Invalid format: ${FORMAT}. Use json or env." >&2
  exit 1
fi

if [[ -n "${UPDATE_FILE}" ]]; then
  if ! command -v jq >/dev/null 2>&1; then
    echo "jq is required for --update mode" >&2
    exit 1
  fi
  if [[ ! -f "${UPDATE_FILE}" ]]; then
    echo "Target settings file not found: ${UPDATE_FILE}" >&2
    exit 1
  fi
  tmp="$(mktemp)"
  jq \
    --arg profile "${PROFILE}" \
    --arg token_path "${token_path}" \
    --arg key_path "${private_key_path}" \
    --arg use_instance_principal_b64 "$(b64 "false")" \
    --arg use_session_token_b64 "$(b64 "true")" \
    --arg tenancy_b64 "$(b64 "${tenancy}")" \
    --arg region_b64 "$(b64 "${region}")" \
    --arg fp_b64 "$(b64 "${fingerprint}")" \
    --arg token_b64 "${session_token_b64}" \
    --arg key_b64 "${session_key_b64}" \
    '
      .oci_session_profile = $profile
      | .oci_session_token_path = $token_path
      | .oci_session_private_key_path = $key_path
      | .kustomize_substitutions = ((.kustomize_substitutions // {}) + {
          "USE_INSTANCE_PRINCIPAL_B64": $use_instance_principal_b64,
          "USE_SESSION_TOKEN_B64": $use_session_token_b64,
          "OCI_TENANCY_ID_B64": $tenancy_b64,
          "OCI_CREDENTIALS_FINGERPRINT_B64": $fp_b64,
          "OCI_REGION_B64": $region_b64,
          "OCI_SESSION_TOKEN_B64": $token_b64,
          "OCI_SESSION_PRIVATE_KEY_B64": $key_b64
        })
    ' "${UPDATE_FILE}" > "${tmp}"
  mv "${tmp}" "${UPDATE_FILE}"
  echo "Updated ${UPDATE_FILE} for profile ${PROFILE}"
  exit 0
fi

cat <<EOF
{
  "oci_session_profile": "${PROFILE}",
  "oci_session_token_path": "${token_path}",
  "oci_session_private_key_path": "${private_key_path}",
  "kustomize_substitutions": {
    "USE_INSTANCE_PRINCIPAL_B64": "$(b64 "false")",
    "USE_SESSION_TOKEN_B64": "$(b64 "true")",
    "OCI_TENANCY_ID_B64": "$(b64 "${tenancy}")",
    "OCI_CREDENTIALS_FINGERPRINT_B64": "$(b64 "${fingerprint}")",
    "OCI_REGION_B64": "$(b64 "${region}")",
    "OCI_SESSION_TOKEN_B64": "${session_token_b64}",
    "OCI_SESSION_PRIVATE_KEY_B64": "${session_key_b64}"
  }
}
EOF
