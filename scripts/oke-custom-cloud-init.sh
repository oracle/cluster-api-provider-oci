#!/bin/bash
# This script is a custom cloud init which has to be passed during oke nodepool creation while using CAPI
# CAPI needs Kubernetes node provider ID to be of format oci://<instance-id>, by default OKE sets it as <instance-id>
curl --fail -H "Authorization: Bearer Oracle" -L0 http://169.254.169.254/opc/v2/instance/metadata/oke_init_script | base64 --decode >/var/run/oke-init.sh
provider_id=$(curl --fail -H "Authorization: Bearer Oracle" -L0 http://169.254.169.254/opc/v2/instance/id)
bash /var/run/oke-init.sh --kubelet-extra-args "--provider-id=oci://$provider_id"
