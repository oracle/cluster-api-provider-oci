# Increase boot volume

The default boot volume size of worker nodes is 50 GB. The following steps needs to be followed
to increase the boot volume size.

## Increase the boot volume size in spec

The following snippet shows how to increase the boot volume size of the instances.

```yaml
kind: OCIManagedMachinePool
spec:
  nodeSourceViaImage:
    bootVolumeSizeInGBs: 100
```

## Extend the root partition

In order to take advantage of the larger size, you need to [extend the partition for the boot volume][boot-volume-extension].
Custom cloud init scripts can be used for the same. The following cloud init script extends the root volume.

```bash
#!/bin/bash

# DO NOT MODIFY
curl --fail -H "Authorization: Bearer Oracle" -L0 http://169.254.169.254/opc/v2/instance/metadata/oke_init_script | base64 --decode >/var/run/oke-init.sh

## run oke provisioning script
bash -x /var/run/oke-init.sh

### adjust block volume size
/usr/libexec/oci-growfs -y

touch /var/log/oke.done
```

Encode the file contents into a base64 encoded value as follows.
```bash
cat cloud-init.sh | base64 -w 0
```

Add the value in the following `OCIManagedMachinePool` spec.
```yaml
kind: OCIManagedMachinePool
spec:
  nodeMetadata:
    user_data: "<base64 encoded value from above>"
```

[boot-volume-extension]: https://docs.oracle.com/en-us/iaas/Content/Block/Tasks/extendingbootpartition.htm