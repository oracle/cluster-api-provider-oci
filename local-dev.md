# Local Development

## Tilt

### Tilt Requirements

Install [Tilt](https://docs.tilt.dev/install.html):

- `brew install tilt-dev/tap/tilt` on macOS or Linux.
- `scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket` & `scoop install tilt` on Windows
- for alternatives you can follow the installation instruction for [macOS](https://docs.tilt.dev/install.html#macos), [Linux](https://docs.tilt.dev/install.html#linux) or [Windows](https://docs.tilt.dev/install.html#windows)

After the installation is done, verify that you have installed it correctly with: `tilt version`.

Install Helm:

- brew install helm on MacOS
- `choco install kubernetes-helm` on Windows 
- [Install Instruction](https://helm.sh/docs/intro/install/#from-source-linux-macos) on Linux

You would require installation of Helm for successfully setting up Tilt.

### Using Tilt

From the root of the CAPOCI repository and after configuring the [environment
variables](https://oracle.github.io/cluster-api-provider-oci/gs/install-cluster-api.html#configure-authentication).
One extra ENV that needs to be set is `OCI_CREDENTIALS_KEY_PATH`. This should point to your OCI private PEM file.
Once set you can run the following to generate your `tilt-settings.json` file:

```
cat <<EOF > tilt-settings.json
{
  "kustomize_substitutions": {
      "OCI_TENANCY_ID_B64": "$(echo "${OCI_TENANCY_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "OCI_CREDENTIALS_FINGERPRINT_B64": "$(echo "${OCI_CREDENTIALS_FINGERPRINT}" | tr -d '\n' | base64 | tr -d '\n')",
      "OCI_USER_ID_B64": "$(echo "${OCI_USER_ID}" | tr -d '\n' | base64 | tr -d '\n')",
      "OCI_REGION_B64": "$(echo "${OCI_REGION}" | tr -d '\n' | base64 | tr -d '\n')",
      "OCI_CREDENTIALS_KEY_B64": "$(base64 < "${OCI_CREDENTIALS_KEY_PATH}" | tr -d '\n')"

  }
}
EOF
```

### OCI Session Token Auth with Tilt

You can run Tilt with OCI session-token auth instead of legacy API keys.

1. Create or refresh your OCI CLI session:

```
oci session authenticate --profile DEFAULT --region <region>
```

2. Find your active profile values in `~/.oci/config` and set these ENV vars:
`OCI_TENANCY_ID`, `OCI_REGION`, and `OCI_CREDENTIALS_FINGERPRINT`.

3. Generate or refresh `tilt-settings.json` with your OCI profile:

```
scripts/oci-profile-to-tilt.sh --profile DEFAULT --update tilt-settings.json
```

This command reads `~/.oci/config`, updates the session-token substitutions in `tilt-settings.json`, and preserves existing fields such as `extra_args`. Run it again whenever you refresh your OCI CLI session token.

If you prefer to create the file from scratch, you can use:

```
cat <<EOF > tilt-settings.json
{
  "oci_session_profile": "DEFAULT",
  "oci_session_token_path": "$HOME/.oci/sessions/DEFAULT/token",
  "oci_session_private_key_path": "$HOME/.oci/sessions/DEFAULT/oci_api_key.pem",
  "kustomize_substitutions": {
    "USE_INSTANCE_PRINCIPAL_B64": "$(echo "false" | tr -d '\n' | base64 | tr -d '\n')",
    "USE_SESSION_TOKEN_B64": "$(echo "true" | tr -d '\n' | base64 | tr -d '\n')",
    "OCI_SESSION_TOKEN_B64": "$(base64 < "$HOME/.oci/sessions/DEFAULT/token" | tr -d '\n')",
    "OCI_SESSION_PRIVATE_KEY_B64": "$(base64 < "$HOME/.oci/sessions/DEFAULT/oci_api_key.pem" | tr -d '\n')"
  }
}
EOF
```

Notes:
- Keep `OCI_REGION`, `OCI_CREDENTIALS_FINGERPRINT`, and `OCI_TENANCY_ID` aligned with `oci_session_profile` in `~/.oci/config`.
- Tilt now defaults to session-token auth when `oci_session_profile` is set and validates these values match the selected profile.
- If you omit `OCI_*_B64` values above, Tilt will read tenancy/region/fingerprint from the selected OCI profile.
- Run the `oci-session-refresh` Tilt resource after updating the file so the controller picks up the new token.

4. Start Tilt:

```
make tilt-up
```

Tilt exposes a manual `oci-session-refresh` resource that refreshes the OCI CLI session token and reapplies the `auth-config` secret.

Refresh runbook:

```
tilt trigger oci-session-refresh
```

Caveats:
- `oci_session_token_path` and `oci_session_private_key_path` must be set for the refresh resource.
- Session-token auth depends on OCI CLI session state on your local machine.
- CAPOCI uses the OCI SDK refreshable session-token provider; if auth errors persist after refresh, restart the CAPOCI controller pod once.

To build a kind cluster and start Tilt, just run: 

```
make tilt-up
```

Once your kind management cluster is up and running, you can deploy a [workload cluster](https://oracle.github.io/cluster-api-provider-oci/gs/create-workload-cluster.html).

To tear down the kind cluster built by the command above you will need to
delete your workload cluster:

```
kubectl delete cluster <CLUSTER_NAME>
```

then delete your local management cluster:

```
kind delete cluster
```
