# Create a workload cluster with MachineHealthChecks (MHC)

To better understand MachineHealthChecks please read over [the Cluster-API book][mhc]
and make sure to read the [limitations][mhc-limitations] sections.

## Create a new workload cluster with MHC

In the project's code repository we provide an [example template][mhc-template] that sets up two MachineHealthChecks
at workload creation time. The example sets up two MHCs to allow differing remediation values:

- `control-plane-unhealthy-5m` setups a health check for the control plane machines
- `md-unhealthy-5m` sets up a health check for the workload machines

> NOTE: As a part of the example template the MHCs will start remediating nodes that are `not ready` after 10 minutes.
In order prevent this side effect make sure to [install your CNI][install-a-cni-provider] once the API is available.
This will move the machines into a `Ready` state.

## Add MHC to existing workload cluster

Another approach is to install MHC after the cluster is up and healthy (aka Day-2 Operation). This can prevent
machine remediation while setting up the cluster.

Adding the MHC to either control-plane or machine is a multistep process. The steps are run on specific clusters
(e.g. management cluster, workload cluster):
1. Update the spec for future instances (management cluster)
2. Add label to existing nodes (workload cluster)
3. Add the MHC (management cluster)

### Add control-plane MHC

#### Update control plane spec
We need to add the `controlplane.remediation` label to the `KubeadmControlPlane`.

Create a file named `control-plane-patch.yaml` that has this content:
```yaml
spec:
  machineTemplate:
    metadata:
      labels:
        controlplane.remediation: ""
```

Then on the management cluster run
`kubectl patch KubeadmControlPlane <your-cluster-name>-control-plane --patch-file control-plane-patch.yaml --type=merge`.

#### Add label to existing nodes

Then on the workload cluster add the new label to any existing control-plane node(s)
`kubectl label node <control-plane-name> controlplane.remediation=""`. This will prevent the `KubeadmControlPlane` provisioning
new nodes once the MHC is deployed.

#### Add the MHC

Finally, create a file named `control-plane-mhc.yaml` that has this content: 
```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineHealthCheck
metadata:
  name: "<your-cluster-name>-control-plane-unhealthy-5m"
spec:
  clusterName: "<your-cluster-name>"
  maxUnhealthy: 100%
  nodeStartupTimeout: 10m
  selector:
    matchLabels:
      controlplane.remediation: ""
  unhealthyConditions:
    - type: Ready
      status: Unknown
      timeout: 300s
    - type: Ready
      status: "False"
      timeout: 300s
```

Then on the management cluster run `kubectl apply -f control-plane-mhc.yaml`.

Then run `kubectl get machinehealthchecks` to check your MachineHealthCheck sees the expected machines.

### Add machine MHC

#### Update machine spec

We need to add the `machine.remediation` label to the `MachineDeployment`.

Create a file named `machine-patch.yaml` that has this content:
```yaml
spec:
  template:
    metadata:
      labels:
        machine.remediation: ""
```

Then on the management cluster run
`kubectl patch MachineDeployment oci-cluster-stage-md-0 --patch-file machine-patch.yaml --type=merge`.

#### Add label to existing nodes

Then on the workload cluster add the new label to any existing control-plane node(s)
`kubectl label node <machine-name> machine.remediation=""`. This will prevent the `MachineDeployment` provisioning
new nodes once the MHC is deployed.

#### Add the MHC

Finally, create a file named `machine-mhc.yaml` that has this content:
```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineHealthCheck
metadata:
  name: "<your-cluster-name>-stage-md-unhealthy-5m"
spec:
  clusterName: "oci-cluster-stage"
  maxUnhealthy: 100%
  nodeStartupTimeout: 10m
  selector:
    matchLabels:
      machine.remediation: ""
  unhealthyConditions:
    - type: Ready
      status: Unknown
      timeout: 300s
    - type: Ready
      status: "False"
      timeout: 300s
```

Then on the management cluster run `kubectl apply -f machine-mhc.yaml`.

Then run `kubectl get machinehealthchecks` to check your MachineHealthCheck sees the expected machines.

[install-a-cni-provider]: ../gs/create-workload-cluster.md#install-a-cni-provider
[mhc]: https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking.html
[mhc-limitations]: https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/healthchecking.html#limitations-and-caveats-of-a-machinehealthcheck
[mhc-template]: https://github.com/oracle/cluster-api-provider-oci/blob/main/templates/cluster-template-healcheck.yaml