
envsubst_cmd = "./hack/tools/bin/envsubst"
kubectl_cmd = "./hack/tools/bin/kubectl"
helm_cmd = "./hack/tools/bin/helm"
tools_bin = "./hack/tools/bin"

update_settings(k8s_upsert_timeout_secs = 60)  # on first tilt up, often can take longer than 30 seconds

#Add tools to path
os.putenv("PATH", os.getenv("PATH") + ":" + tools_bin)

legacy_auth_keys = ["OCI_TENANCY_ID", "OCI_USER_ID", "OCI_CREDENTIALS_FINGERPRINT", "OCI_REGION", "OCI_CREDENTIALS_KEY"]
session_auth_keys = ["OCI_TENANCY_ID", "OCI_CREDENTIALS_FINGERPRINT", "OCI_REGION", "OCI_SESSION_TOKEN", "OCI_SESSION_PRIVATE_KEY"]

# set defaults
settings = {
    "allowed_contexts": [
        "kind-capoci",
    ],
    "deploy_cert_manager": True,
    "preload_images_for_kind": True,
    "kind_cluster_name": "capoci",
    "capi_version": "v1.11.0",
    "cert_manager_version": "v1.16.2",
    "kubernetes_version": "v1.30.0",
}

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default = {},
))

if settings.get("trigger_mode") == "manual":
    trigger_mode(TRIGGER_MODE_MANUAL)

if "allowed_contexts" in settings:
    allow_k8s_contexts(settings.get("allowed_contexts"))

if "default_registry" in settings:
    default_registry(settings.get("default_registry"))

tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.24.3 as tilt-helper
# Support live reloading with Tilt
RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh && chmod +x /go/bin/dlv && \
    touch /process.txt && chmod 0777 /process.txt `# pre-create PID file to allow even non-root users to run the image`
"""

tilt_dockerfile_header = """
FROM gcr.io/distroless/base:debug as tilt
WORKDIR /
COPY --from=tilt-helper /process.txt .
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY --from=tilt-helper /go/bin/dlv .
COPY manager .
"""

def validate_auth():
    substitutions = settings.get("kustomize_substitutions", {})
    os.environ.update(substitutions)
    for sub in substitutions:
        if sub[-4:] == "_B64":
            os.environ[sub[:-4]] = base64_decode(os.environ[sub])
            print("loaded {} from {}".format(sub[:-4], sub))

    if not os.environ.get("OCI_CREDENTIALS_KEY") and os.environ.get("OCI_CREDENTIALS_KEY_PATH"):
        os.environ["OCI_CREDENTIALS_KEY"] = read_file_from_path(os.environ.get("OCI_CREDENTIALS_KEY_PATH"))

    use_instance_principal = str(os.environ.get("USE_INSTANCE_PRINCIPAL", "false")).strip().lower() == "true"
    use_session_token = str(os.environ.get("USE_SESSION_TOKEN", "false")).strip().lower() == "true"
    if not use_instance_principal and not use_session_token and settings.get("oci_session_profile"):
        # If a session profile is configured, default to session-token mode to avoid silent fallback to API-key auth.
        use_session_token = True
        os.environ["USE_SESSION_TOKEN"] = "true"
        print("oci_session_profile is set; defaulting auth mode to session-token")

    if use_instance_principal:
        return "instance-principal"

    if use_session_token:
        apply_session_profile_defaults()
        missing = [k for k in session_auth_keys if not os.environ.get(k)]
        if missing:
            fail("session-token auth selected but missing kustomize_substitutions values for {}".format(missing))
        return "session-token"

    missing = [k for k in legacy_auth_keys if not os.environ.get(k)]
    if missing:
        fail("legacy API-key auth selected but missing kustomize_substitutions values for {}. Set USE_SESSION_TOKEN_B64 or USE_INSTANCE_PRINCIPAL_B64 to switch modes.".format(missing))
    return "legacy-api-key"

def add_session_token_refresh_resource(auth_mode):
    if auth_mode != "session-token":
        return

    session_profile = settings.get("oci_session_profile", "DEFAULT")
    token_path = expand_path(settings.get("oci_session_token_path"))
    private_key_path = expand_path(settings.get("oci_session_private_key_path"))
    if not token_path or not private_key_path:
        local_resource(
            "oci-session-refresh",
            cmd = "echo 'Set oci_session_token_path and oci_session_private_key_path in tilt-settings.json for session refresh.' >&2; exit 1",
            trigger_mode = TRIGGER_MODE_MANUAL,
            auto_init = False,
            labels = ["cluster-api"],
        )
        return

    refresh_cmd = """set -euo pipefail
if command -v oci >/dev/null 2>&1; then
  if ! oci session refresh --profile '{profile}'; then
    echo "warning: oci session refresh failed for profile '{profile}', continuing with local session files" >&2
  fi
fi
CAPOCI_NAMESPACE="$({kubectl_cmd} get deploy -A -o jsonpath='{{range .items[?(@.metadata.name==\"capoci-controller-manager\") ]}}{{.metadata.namespace}}{{end}}' 2>/dev/null || true)"
if [ -z "$CAPOCI_NAMESPACE" ]; then
  CAPOCI_NAMESPACE="cluster-api-provider-oci-system"
fi
CAPOCI_AUTH_SECRET_NAME="$({kubectl_cmd} get deploy capoci-controller-manager -n "$CAPOCI_NAMESPACE" -o jsonpath='{{.spec.template.spec.volumes[?(@.name==\"auth-config-dir\")].secret.secretName}}' 2>/dev/null || true)"
if [ -z "$CAPOCI_AUTH_SECRET_NAME" ]; then
  CAPOCI_AUTH_SECRET_NAME="capoci-auth-config"
fi
export CAPOCI_NAMESPACE
export CAPOCI_AUTH_SECRET_NAME
export USE_INSTANCE_PRINCIPAL_B64="ZmFsc2U="
export USE_SESSION_TOKEN_B64="dHJ1ZQ=="
export OCI_SESSION_TOKEN_B64="$(base64 < '{token_path}' | tr -d '\\n')"
export OCI_SESSION_PRIVATE_KEY_B64="$(awk 'BEGIN{{in_key=0}}
!in_key {{
  if (match($0, /-----BEGIN [A-Z ]*PRIVATE KEY-----/)) {{
    in_key=1
  }} else {{
    next
  }}
}}
{{
  line=$0
  if (match(line, /-----END [A-Z ]*PRIVATE KEY-----/)) {{
    print substr(line, 1, RSTART + RLENGTH - 1)
    exit
  }}
  print line
}}' '{private_key_path}' | base64 | tr -d '\\n')"
{envsubst_cmd} < config/default/credentials.yaml \
  | sed "s/^  name: auth-config$/  name: $CAPOCI_AUTH_SECRET_NAME/" \
  | sed "s/^  namespace: system$/  namespace: $CAPOCI_NAMESPACE/" \
  | {kubectl_cmd} apply -f -
""".format(
        profile = session_profile,
        token_path = token_path,
        private_key_path = private_key_path,
        envsubst_cmd = envsubst_cmd,
        kubectl_cmd = kubectl_cmd,
    )
    local_resource(
        "oci-session-refresh",
        cmd = refresh_cmd,
        trigger_mode = TRIGGER_MODE_MANUAL,
        auto_init = True,
        labels = ["cluster-api"],
    )

# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)

# deploy CAPI
def deploy_capi():
    version = settings.get("capi_version")
    capi_uri = "https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | {} apply -f -".format(capi_uri, envsubst_cmd, kubectl_cmd)
    local(cmd, quiet = False)
    if settings.get("extra_args"):
        extra_args = settings.get("extra_args")
        if extra_args.get("core"):
            core_extra_args = extra_args.get("core")
            if core_extra_args:
                for namespace in ["capi-system", "capi-webhook-system"]:
                    patch_args_with_extra_args(namespace, "capi-controller-manager", core_extra_args)
        if extra_args.get("kubeadm-bootstrap"):
            kb_extra_args = extra_args.get("kubeadm-bootstrap")
            if kb_extra_args:
                patch_args_with_extra_args("capi-kubeadm-bootstrap-system", "capi-kubeadm-bootstrap-controller-manager", kb_extra_args)

def patch_args_with_extra_args(namespace, name, extra_args):
    args_str = str(local("{} get deployments {} -n {} -o jsonpath={{.spec.template.spec.containers[1].args}}".format(kubectl_cmd, name, namespace)))
    args_to_add = [arg for arg in extra_args if arg not in args_str]
    if args_to_add:
        args = args_str[1:-1].split()
        args.extend(args_to_add)
        patch = [{
            "op": "replace",
            "path": "/spec/template/spec/containers/1/args",
            "value": args,
        }]
        local("{} patch deployment {} -n {} --type json -p='{}'".format(kubectl_cmd, name, namespace, str(encode_json(patch)).replace("\n", "")))

# Build CAPOCI and add feature gates
def capoci():
    # Apply the kustomized yaml for this provider
    yaml = str(kustomizesub("./config/default"))

    # add extra_args if they are defined
    if settings.get("extra_args"):
        oci_extra_args = settings.get("extra_args").get("oci")
        if oci_extra_args:
            yaml_dict = decode_yaml_stream(yaml)
            append_arg_for_container_in_deployment(yaml_dict, "capoci-controller-manager", "capoci-system", "cluster-api-oci-controller", oci_extra_args)
            yaml = str(encode_yaml_stream(yaml_dict))
            yaml = fixup_yaml_empty_arrays(yaml)

    # Set up a local_resource build of the provider's manager binary.
    local_resource(
        "manager",
        cmd = 'mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags \'-extldflags "-static"\' -o .tiltbuild/manager',
        deps = ["api", "cloud", "config", "controllers", "exp", "feature", "pkg", "go.mod", "go.sum", "main.go", "auth-config.yaml"],
        labels = ["cluster-api"],
    )

    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        tilt_dockerfile_header,
    ])

    entrypoint = ["sh", "/start.sh", "/manager"]
    extra_args = settings.get("extra_args")
    if extra_args:
        entrypoint.extend(extra_args)

    # Set up an image build for the provider. The live update configuration syncs the output from the local_resource
    # build into the container.
    docker_build(
        ref = "ghcr.io/oracle/cluster-api-oci-controller-amd64:dev",
        context = "./.tiltbuild/",
        dockerfile_contents = dockerfile_contents,
        target = "tilt",
        entrypoint = entrypoint,
        only = "manager",
        live_update = [
            sync(".tiltbuild/manager", "/manager"),
            run("sh /restart.sh"),
        ],
        ignore = ["templates"],
        network = "host",
    )

    #secret_settings(disable_scrub=True)
    k8s_yaml(blob(yaml))


def append_arg_for_container_in_deployment(yaml_stream, name, namespace, contains_image_name, args):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if contains_image_name in container.get("image"):
                    container.get("args").extend(args)

def fixup_yaml_empty_arrays(yaml_str):
    yaml_str = yaml_str.replace("conditions: null", "conditions: []")
    return yaml_str.replace("storedVersions: null", "storedVersions: []")

def waitforsystem():
    local(kubectl_cmd + " wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-bootstrap-system")
    local(kubectl_cmd + " wait --for=condition=ready --timeout=300s pod --all -n capi-kubeadm-control-plane-system")
    local(kubectl_cmd + " wait --for=condition=ready --timeout=300s pod --all -n capi-system")

def base64_encode(to_encode):
    encode_blob = local("echo '{}' | tr -d '\n' | base64 - | tr -d '\n'".format(to_encode), quiet = True, echo_off = True)
    return str(encode_blob)

def base64_encode_file(path_to_encode):
    encode_blob = local("base64 < {} | tr -d '\n'".format(shell_single_quote(path_to_encode)), quiet = True)
    return str(encode_blob)

def read_file_from_path(path_to_read):
    str_blob = local("cat {}".format(shell_single_quote(path_to_read)), quiet = True)
    return str(str_blob)

def read_private_key_pem_from_path(path_to_read):
    awk_cmd = """awk 'BEGIN{{in_key=0}}
!in_key {{
  if (match($0, /-----BEGIN [A-Z ]*PRIVATE KEY-----/)) {{
    in_key=1
  }} else {{
    next
  }}
}}
{{
  line=$0
  if (match(line, /-----END [A-Z ]*PRIVATE KEY-----/)) {{
    print substr(line, 1, RSTART + RLENGTH - 1)
    exit
  }}
  print line
}}' {path}""".format(path = shell_single_quote(path_to_read))
    pem = str(local(awk_cmd, quiet = True, echo_off = True)).strip()
    if not pem:
        fail("no PEM private key block found in {}".format(path_to_read))
    return pem + "\n"

def shell_single_quote(s):
    return "'" + str(s).replace("'", "'\"'\"'") + "'"

def expand_path(path):
    if path == None:
        return ""
    path_str = str(path).strip()
    if path_str == "":
        return ""
    home = str(os.getenv("HOME", ""))
    if path_str[0:2] == "~/":
        return home + path_str[1:]
    return path_str.replace("$HOME", home).replace("${HOME}", home)

def read_oci_profile_value(profile, key):
    config_file = expand_path("~/.oci/config")
    if not config_file:
        return ""
    profile_name = str(profile).strip()
    key_name = str(key).strip()
    if not profile_name or not key_name:
        return ""
    awk_cmd = """awk '
$0 == "[{profile}]" {{ in_profile=1; next }}
/^\\[/ {{ in_profile=0 }}
in_profile && $0 ~ /^[[:space:]]*{key}[[:space:]]*=/ {{
  line=$0
  sub(/^[[:space:]]*{key}[[:space:]]*=[[:space:]]*/, "", line)
  print line
  exit
}}
' {config_file}""".format(
        profile = profile_name,
        key = key_name,
        config_file = shell_single_quote(config_file),
    )
    return str(local(awk_cmd, quiet = True, echo_off = True)).strip()

def set_env_if_missing(name, value):
    if value and not str(os.environ.get(name, "")).strip():
        os.environ[name] = str(value).strip()

def ensure_env_matches_profile(name, profile_value, profile_name):
    env_val = str(os.environ.get(name, "")).strip()
    if not profile_value or not env_val:
        return
    if env_val != profile_value:
        fail("{} in kustomize_substitutions ({}) does not match {} profile {} value ({}).".format(name, env_val, profile_name, name.lower(), profile_value))

def apply_session_profile_defaults():
    profile = str(settings.get("oci_session_profile", "")).strip()
    if not profile:
        return

    profile_tenancy = read_oci_profile_value(profile, "tenancy")
    profile_region = read_oci_profile_value(profile, "region")
    profile_fingerprint = read_oci_profile_value(profile, "fingerprint")
    profile_token_path = read_oci_profile_value(profile, "security_token_file")
    profile_key_path = read_oci_profile_value(profile, "key_file")

    set_env_if_missing("OCI_TENANCY_ID", profile_tenancy)
    set_env_if_missing("OCI_REGION", profile_region)
    set_env_if_missing("OCI_CREDENTIALS_FINGERPRINT", profile_fingerprint)

    ensure_env_matches_profile("OCI_TENANCY_ID", profile_tenancy, profile)
    ensure_env_matches_profile("OCI_REGION", profile_region, profile)
    ensure_env_matches_profile("OCI_CREDENTIALS_FINGERPRINT", profile_fingerprint, profile)

    token_path = expand_path(settings.get("oci_session_token_path"))
    private_key_path = expand_path(settings.get("oci_session_private_key_path"))
    if not token_path:
        token_path = expand_path(profile_token_path)
    if not private_key_path:
        private_key_path = expand_path(profile_key_path)

    if token_path and not os.environ.get("OCI_SESSION_TOKEN"):
        os.environ["OCI_SESSION_TOKEN"] = read_file_from_path(token_path)
    if os.environ.get("OCI_SESSION_TOKEN"):
        os.environ["OCI_SESSION_TOKEN_B64"] = base64_encode(os.environ["OCI_SESSION_TOKEN"])
    if private_key_path:
        os.environ["OCI_SESSION_PRIVATE_KEY"] = read_private_key_pem_from_path(private_key_path)
        os.environ["OCI_SESSION_PRIVATE_KEY_B64"] = base64_encode(os.environ["OCI_SESSION_PRIVATE_KEY"])

def base64_decode(to_decode):
    # Use -D for macOS (BSD), --decode - for Linux (GNU)
    # Detect OS using uname command
    os_check = local("uname", quiet = True)
    if "Darwin" in str(os_check):
        # macOS BSD base64
        decode_blob = local("echo '{}' | base64 -D".format(to_decode), quiet = True, echo_off = True)
    else:
        # Linux GNU base64
        decode_blob = local("echo '{}' | base64 --decode -".format(to_decode), quiet = True, echo_off = True)
    return str(decode_blob)

def kustomizesub(folder):
    yaml = local('bash hack/kustomize-sub.sh {}'.format(folder), quiet=True)
    return yaml

##############################
# Actual work happens here
##############################

auth_mode = validate_auth()
add_session_token_refresh_resource(auth_mode)

include_user_tilt_files()

load("ext://cert_manager", "deploy_cert_manager")

if settings.get("deploy_cert_manager"):
    deploy_cert_manager()

deploy_capi()

capoci()

waitforsystem()
