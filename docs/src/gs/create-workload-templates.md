# Create Workload Templates for Oracle Cloud Infrastructure

You can create workload clusters based on template files or you can also save the templates in `ConfigMaps` that you can then reuse.

## Creating cluster templates ConfigMaps

1. Create a cluster template for Oracle Linux:

```shell
kubectl create cm oracletemplate --from-file=template=templates/cluster-template-oraclelinux.yaml
```

1. Create a cluster template for Ubuntu:

```shell
kubectl create cm ubuntutemplate --from-file=template=templates/cluster-template.yaml
```

You can then reuse the `ConfigMap` to create your clusters. For example, to create a workload cluster using Oracle Linux, you can create it as follows:

```shell
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<oracle-linux-custom-image-id> \
OCI_SHAPE=VM.Standard.E4.Flex \
OCI_SHAPE_OCPUS=1 \
OCI_SHAPE_MEMORY_IN_GBS= \
OCI_SSH_KEY=<ssh-key>  \
CONTROL_PLANE_MACHINE_COUNT=1 \
KUBERNETES_VERSION=v1.20.10 \
NAMESPACE=default \
WORKER_MACHINE_COUNT=1 \
clusterctl generate cluster <cluster-name>\
--from-config-map oracletemplate | kubectl apply -f -
```

Likewise, to create a workload cluster using Ubuntu:

```shell
OCI_COMPARTMENT_ID=<compartment-id> \
OCI_IMAGE_ID=<ubuntu-custom-image-id> \
OCI_SHAPE=VM.Standard.E4.Flex \
OCI_SHAPE_OCPUS=1 \
OCI_SHAPE_MEMORY_IN_GBS= \
OCI_SSH_KEY=<ssh-key>  \
CONTROL_PLANE_MACHINE_COUNT=1 \
KUBERNETES_VERSION=v1.20.10 \
NAMESPACE=default \
WORKER_MACHINE_COUNT=1 \
clusterctl generate cluster <cluster-name>\
--from-config-map ubuntutemplate | kubectl apply -f -
```
