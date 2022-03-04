# Provision a management cluster

Cluster API Provider for Oracle Cloud Infrastructure is installed into an existing Kubernetes cluster, called the [management cluster][management_cluster].

You may use [kind][kind] for experimental purposes or for creating a [local bootstrap cluster][bootstrap_cluster] which you will then use to provision a target management cluster.

* [Create a local management or bootstrap cluster with kind](./mgmt/mgmt-kind.md)

For a more durable environment, we recommend using a managed Kubernetes service such as [Oracle Container Engine for Kubernetes][oke] (OKE).

* [Create a management cluster with OKE](./mgmt/mgmt-oke.md)

[bootstrap_cluster]: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-andor-configure-a-kubernetes-cluster
[kind]: https://kind.sigs.k8s.io/
[management_cluster]: https://cluster-api.sigs.k8s.io/user/concepts.html#management-cluster
[oke]: https://docs.oracle.com/en-us/iaas/Content/ContEng/home.htm
