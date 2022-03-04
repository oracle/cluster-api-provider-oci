# Getting started with Kubernetes Cluster API Provider for Oracle Cloud Infrastructure

Before deploying the Cluster API Provider for Oracle Cloud Infrastructure (CAPOCI), you must first configure the 
required Identity and Access Management (IAM) policies:

![CAPOCI Installation Process](../images/iam.svg)

The following deployment options are available:

- [Getting started with Kubernetes Cluster API Provider for Oracle Cloud Infrastructure](#getting-started-with-kubernetes-cluster-api-provider-for-oracle-cloud-infracture)
  - [Setting up a non-production management cluster](#setting-up-a-non-production-management-cluster)
  - [Setting up a management cluster using an initial bootstrap cluster](#setting-up-a-management-cluster-using-an-initial-bootstrap-cluster)
  - [Setting up a management cluster using OKE](#setting-up-a-management-cluster-using-oke)
  - [Setting up a management cluster using a 3rd party Kubernetes cluster](#setting-up-a-management-cluster-using-a-3rd-party-kubernetes-cluster)

The following workflow diagrams provide a high-level overview of each deployment method described above:

## Setting up a non-production management cluster

![CAPOCI Installation Process](../images/nonprod.svg)

## Setting up a management cluster using an initial bootstrap cluster

![CAPOCI Installation Process](../images/bootstrap.svg)

## Setting up a management cluster using OKE

![CAPOCI Installation Process](../images/oke.svg)

## Setting up a management cluster using a 3rd party Kubernetes cluster

![CAPOCI Installation Process](../images/3rdparty.svg)

Complete the following steps in order to install and use CAPOCI:

1. Choose your management cluster. You can use [kind][kind], [OKE][oke] or any other compliant Kubernetes clusters.
1. [Prepare custom machine images][custom-machine-images]
1. [Configure users and policies for the management cluster if required][iam]
1. [Provision a management cluster][provision-management-cluster]. You can use [kind][kind], [OKE][oke] or any other compliant Kubernetes clusters.
1. Install the necessary tools:
   - [OCI CLI][oci-cli]
   - [`clusterctl`][clusterctl]
   - [`kubectl`][kubectl]
1. [Configure IAM for the workload cluster](iam/iam-self-provisioned.md).
1. [Install Kubernetes Cluster API][install-cluster-api] for Oracle Cloud Infrastructure (CAPOCI) in the ***management cluster***.
1. [Create a workload cluster][create-workload-cluster].

[cluster-api]: https://cluster-api.sigs.k8s.io/
[clusterctl]: https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl
[iam]: ./iam.md
[custom-machine-images]: ./custom-machine-images.md
[provision-management-cluster]: ./provision-mgmt-cluster.md
[install-cluster-api]: ./install-cluster-api.md
[create-workload-cluster]: ./create-workload-cluster.md
[kind]: https://kind.sigs.k8s.io/
[kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[oke]: https://docs.oracle.com/en-us/iaas/Content/ContEng/home.htm
[oci-cli]: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/cliconcepts.htm
