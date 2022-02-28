# Provision a management cluster using [kind][kind]

1. Create the cluster

    ```shell
      kind create cluster
    ```

1. Configure access

    ```shell
      kubectl config set-context kind-kind
    ```

[bootstrap_cluster]: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-andor-configure-a-kubernetes-cluster
[kind]: https://kind.sigs.k8s.io/
