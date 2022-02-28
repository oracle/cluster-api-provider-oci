# Configure Custom Machine Images

An image is a template of a virtual hard drive. It determines the operating system and other software for a compute instance. In order to use CAPOCI, you must prepare one or more custom images which have all the necessary Kubernetes components pre-installed. The custom image(s) will then be used to instantiate the Kubernetes nodes.

## Building a custom image

To create your own custom image in your Oracle Cloud Infrastructure (OCI) tenancy, navigate to [The Image Builder Book][image_builder_book] and follow the instructions for OCI.

[image_builder_book]: https://image-builder.sigs.k8s.io/capi/providers/oci.html
