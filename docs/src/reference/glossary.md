# Table of Contents

[A](#a) | [B](#b) | [C](#c) | [D](#d) | [H](#h) | [I](#i) | [K](#k) | [M](#m) | [N](#n) | [O](#o) | [P](#p) | [R](#r) | [S](#s) | [T](#t) | [W](#w)

# A
---

### AD

Or __Availability Domain__ 

One or more isolated, fault-tolerant Oracle data centers that host cloud resources such as instances, volumes, and subnets. A region contains one or more availability domains.

# C
---

### CNI

Or __Container Network Interface__

A [Cloud Native Computing Foundation](https://cncf.io) project that  consists of a specification and libraries for 
writing plugins to configure network interfaces in Linux containers, along with a number of supported plugins.


# F
---

### FD

Or __Fault Domain__ 

A logical grouping of hardware and infrastructure within an [availability domain](#ad). Fault domains isolate resources during hardware failure or unexpected software changes.

# I
---

### Internet Gateway

An Internet Gateway is an optional virtual router you can add to your [VCN](#vcn) to enable direct connectivity to the Internet. It supports connections initiated from within the VCN (egress) and connections initiated from the Internet (ingress).

# N
---

### NAT Gateway

A NAT Gateway gives cloud resources without public IP addresses access to the Internet without exposing those resources to incoming internet connections. A NAT Gateway can be added to a [VCN](#vcn) to give instances in a private subnet access to the Internet.

### NSG

Or __Network Security Group__

A [Network security group (NSG)][oci_nsg] acts as a virtual firewall for your compute instances and other kinds of resources. An NSG consists of a set of ingress and egress security rules that apply only to a set of VNICs of your choice in a single VCN (for example: all compute instances that act as web servers in the web tier of a multi-tier application in your VCN).

# R
---

### Region

Oracle Cloud Infrastructure is hosted in regions and availability domains. A [region][oci_region] is a localized geographic area, and an [availability domain](#ad) is one or more data centers located within a region. A region is composed of one or more availability domains.

# S
---

### Service Gateway

A service gateway lets your [VCN](#vcn) privately access specific Oracle services without exposing the data to the public Internet. No [Internet Gateway](#internet-gateway) or [NAT](#nat-gateway) is required to reach those specific services. The resources in the VCN can be in a private subnet and use only private IP addresses. The traffic from the VCN to the Oracle service travels over the Oracle network fabric and never traverses the Internet.

# V
---

### VCN

Or __Virtual Cloud Network__

A [VCN][oci_vcn] is a software-defined network that you set up in the Oracle Cloud Infrastructure data centers in a particular [region](#region).


[oci_nsg]: https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/networksecuritygroups.htm
[oci_region]: https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm
[oci_vcn]: https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/managingVCNs_topic-Overview_of_VCNs_and_Subnets.htm#Overview