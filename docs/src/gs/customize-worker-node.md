# Customizing worker nodes
## Configure user managed boot volume encryption
Use the following configuration in `OCIMachineTemplate` to use a [customer
managed boot volume encryption key][customer_managed_keys].
```yaml
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
spec:
  template:
    spec:
      instanceSourceViaImageConfig:
        kmsKeyId: <kms-key-id>
```

## Configure shielded instances
Use the following configuration in `OCIMachineTemplate` to create [shielded instances][shielded_instances].
Below example is for an AMD based VM. Please read the [CAPOCI github page][github_capoci_types] PlatformConfig struct
for an enumeration of all the possible configurations.

```yaml
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
spec:
  template:
    spec:
      platformConfig:
        platformConfigType: "AMD_VM"
        amdVmPlatformConfig:
          isSecureBootEnabled: true
          isTrustedPlatformModuleEnabled: true
          isMeasuredBootEnabled: true
```

## Configure confidential instances
Use the following configuration in `OCIMachineTemplate` to create [confidential instances][confidential_instances].
Below example is for an AMD based VM. Please read the [CAPOCI github page][github_capoci_types] PlatformConfig struct
for an enumeration of all the possible configurations.

```yaml
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
spec:
  template:
    spec:
      platformConfig:
        platformConfigType: "AMD_VM"
        amdVmPlatformConfig:
          isMemoryEncryptionEnabled: true
```
## Configure preemptible instances
Use the following configuration in `OCIMachineTemplate` to create [preemtible instances][preemptible_instances].

```yaml
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
spec:
  template:
    spec:
      preemptibleInstanceConfig:
        terminatePreemptionAction:
          preserveBootVolume: false
```

## Configure capacity reservation
Use the following configuration in `OCIMachineTemplate` to use [capacity reservations][capacity_reservations].

```yaml
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
spec:
  template:
    spec:
      capacityReservationId: <capacity-reservation-id>
```

## Configure Oracle Cloud Agent plugins
Use the following configuration in `OCIMachineTemplate` to configure [Oracle Cloud Agent plugins][cloud_agent_plugins].
The example below enables Bastion plugin.

```yaml
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
spec:
  template:
    spec:
      agentConfig:
        pluginsConfigs:
          - name: "Bastion"
            desiredState: "ENABLED"
```

## Configure Burstable Instances
Use the following configuration in `OCIMachineTemplate` to configure [Burstable Instance][burstable_instances].
The following values are supported for `baselineOcpuUtilization`.
* BASELINE_1_8 - baseline usage is 1/8 of an OCPU.
* BASELINE_1_2 - baseline usage is 1/2 of an OCPU.
* BASELINE_1_1 - baseline usage is an entire OCPU. This represents a non-burstable instance.

```yaml
kind: OCIMachineTemplate
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
spec:
  template:
    spec:
      shapeConfig:
        baselineOcpuUtilization: "BASELINE_1_8"
        ocpus: "1"
```

[customer_managed_keys]: https://docs.oracle.com/en-us/iaas/Content/KeyManagement/Tasks/assigningkeys.htm
[shielded_instances]: https://docs.oracle.com/en-us/iaas/Content/Compute/References/shielded-instances.htm
[confidential_instances]: https://docs.oracle.com/en-us/iaas/Content/Compute/References/confidential_compute.htm
[preemptible_instances]: https://docs.oracle.com/en-us/iaas/Content/Compute/Concepts/preemptible.htm#howitworks__using
[cloud_agent_plugins]: https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/manage-plugins.htm
[github_capoci_types]: https://github.com/oracle/cluster-api-provider-oci/blob/main/api/v1beta1/types.go
[capacity_reservations]: https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/reserve-capacity.htm
[burstable_instances]: https://docs.oracle.com/en-us/iaas/Content/Compute/References/burstable-instances.htm