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
      "OCI_CREDENTIALS_KEY_B64": "$(echo "${OCI_CREDENTIALS_KEY_PATH}" | tr -d '\n' | base64 | tr -d '\n')"

  }
}
EOF
```

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