# Advanced Options

## Disable OCI Client initialization on startup

CAPOCI supports setting OCI principals at [cluster level][cluster-identity], hence CAPOCI can be
installed without providing OCI user credentials. The following environment variable need to be exported
to install CAPOCI without providing any OCI credentials.

   ```shell
   export INIT_OCI_CLIENTS_ON_STARTUP=false
   ```

If the above setting is used, and [Cluster Identity][cluster-identity] is not used, the OCICluster will
go into error state, and the following error will show up in the CAPOCI pod logs.

`OCI authentication credentials could not be retrieved from pod or cluster level,please install Cluster API Provider for OCI with OCI authentication credentials or set Cluster Identity in the OCICluster`

## Setup heterogeneous cluster

> This section assumes you have [setup a Windows workload cluster][windows-cluster].

To add Linux nodes to the existing Windows workload cluster use the following YAML as a guide to provision 
just the new Linux machines.

Create a file and call it `cluster-template-windows-calico-heterogeneous.yaml`. Then add the following:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OCIMachineTemplate
metadata:
  name: "${CLUSTER_NAME}-md-0"
spec:
  template:
    spec:
      imageId: "${OCI_IMAGE_ID}"
      compartmentId: "${OCI_COMPARTMENT_ID}"
      shape: "${OCI_NODE_MACHINE_TYPE=VM.Standard.E4.Flex}"
      shapeConfig:
        ocpus: "${OCI_NODE_MACHINE_TYPE_OCPUS=1}"
      metadata:
        ssh_authorized_keys: "${OCI_SSH_KEY}"
      isPvEncryptionInTransitEnabled: ${OCI_NODE_PV_TRANSIT_ENCRYPTION=true}
---
apiVersion: bootstrap.cluster.x-k8s.io/v1alpha4
kind: KubeadmConfigTemplate
metadata:
  name: "${CLUSTER_NAME}-md-0"
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          kubeletExtraArgs:
            cloud-provider: external
            provider-id: oci://{{ ds["id"] }}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: "${CLUSTER_NAME}-md-0"
spec:
  clusterName: "${CLUSTER_NAME}"
  replicas: ${NODE_MACHINE_COUNT}
  selector:
    matchLabels:
  template:
    spec:
      clusterName: "${CLUSTER_NAME}"
      version: "${KUBERNETES_VERSION}"
      bootstrap:
        configRef:
          name: "${CLUSTER_NAME}-md-0"
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
      infrastructureRef:
        name: "${CLUSTER_NAME}-md-0"
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: OCIMachineTemplate
```

Then apply the template
```bash
OCI_IMAGE_ID=<your new linux image OCID> \
OCI_NODE_IMAGE_ID=<your new linux image OCID> \
OCI_COMPARTMENT_ID=<your compartment> \
NODE_MACHINE_COUNT=2 \
OCI_NODE_MACHINE_TYPE=<shape> \
OCI_NODE_MACHINE_TYPE_OCPUS=4 \
OCI_SSH_KEY="<your public ssh key>" \
clusterctl generate cluster <cluster-name> --kubernetes-version <kubernetes-version> \
--target-namespace default \
--from cluster-template-windows-calico-heterogeneous.yaml | kubectl apply -f -
```

After a few minutes the instances will come up and the CNI will be installed.

### Node constraints

All future deployments make sure to setup node constraints using something like [`nodeselctor`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector). Example:

| Windows      | Linux |
| ----------- | ----------- |
| ```nodeSelector: kubernetes.io/os: windows``` | ```nodeSelector:kubernetes.io/os: linux``` |

<br/>
<details>
  <summary>nodeSelector examples - click to expand</summary>

Linux nginx deployment example:
```bash
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nginx-linux
spec:
  selector:
    matchLabels:
      run: my-nginx-linux
  replicas: 2
  template:
    metadata:
      labels:
        run: my-nginx-linux
    spec:
      nodeSelector:
        kubernetes.io/os: linux
      containers:
      - args:
        - /bin/sh
        - -c
        - sleep 3600
        name: nginx
        image: nginx:latest
```

For a Windows deployment example see the [Kubernetes Getting Started: Deploying a Windows workload][windows-kubernetes-deployment] documentation

</details>

Without doing this it is possible that the Kubernetes scheduler will try to deploy your Windows pods onto a Linux worker, or vice versa.

[cluster-identity]: ./multi-tenancy.md
[windows-cluster]: ./create-windows-workload-cluster.md
[windows-kubernetes-deployment]: https://kubernetes.io/docs/concepts/windows/user-guide/#getting-started-deploying-a-windows-workload