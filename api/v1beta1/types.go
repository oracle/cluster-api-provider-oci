/*
 Copyright (c) 2021, 2022 Oracle and/or its affiliates.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package v1beta1

const (
	ControlPlaneRole         = "control-plane"
	ControlPlaneEndpointRole = "control-plane-endpoint"
	WorkerRole               = "worker"
	ServiceLoadBalancerRole  = "service-lb"
	Private                  = "private"
	Public                   = "public"
)

// SubnetRoles a slice of all the subnet roles
var SubnetRoles = [...]Role{ControlPlaneRole, ControlPlaneEndpointRole, WorkerRole, ServiceLoadBalancerRole}

// NetworkDetails defines the configuration options for the network
type NetworkDetails struct {
	SubnetId       *string `json:"subnetId,omitempty"`
	AssignPublicIp bool    `json:"assignPublicIp,omitempty"`
	SubnetName     string  `json:"subnetName,omitempty"`
	NSGId          *string `json:"nsgId,omitempty"`
}

// ShapeConfig defines the configuration options for the compute instance shape
// https://docs.oracle.com/en-us/iaas/api/#/en/iaas/20160918/datatypes/LaunchInstanceShapeConfigDetails
type ShapeConfig struct {
	// The total number of OCPUs available to the instance.
	Ocpus string `json:"ocpus,omitempty"`

	// The total amount of memory available to the instance, in gigabytes.
	MemoryInGBs string `json:"memoryInGBs,omitempty"`

	// The baseline OCPU utilization for a subcore burstable VM instance. Leave this attribute blank for a
	// non-burstable instance, or explicitly specify non-burstable with `BASELINE_1_1`.
	// The following values are supported:
	// - `BASELINE_1_8` - baseline usage is 1/8 of an OCPU.
	// - `BASELINE_1_2` - baseline usage is 1/2 of an OCPU.
	// - `BASELINE_1_1` - baseline usage is an entire OCPU. This represents a non-burstable instance.
	BaselineOcpuUtilization string `json:"baselineOcpuUtilization,omitempty"`
}

// EgressSecurityRule A rule for allowing outbound IP packets.
type EgressSecurityRule struct {

	// Conceptually, this is the range of IP addresses that a packet originating from the instance
	// can go to.
	// Allowed values:
	//   * IP address range in CIDR notation. For example: `192.168.1.0/24` or `2001:0db8:0123:45::/56`
	//     Note that IPv6 addressing is currently supported only in certain regions. See
	//     IPv6 Addresses (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/ipv6.htm).
	//   * The `cidrBlock` value for a Service, if you're
	//     setting up a security list rule for traffic destined for a particular `Service` through
	//     a service gateway. For example: `oci-phx-objectstorage`.
	Destination *string `json:"destination,omitempty"`

	// The transport protocol. Specify either `all` or an IPv4 protocol number as
	// defined in
	// Protocol Numbers (http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml).
	// Options are supported only for ICMP ("1"), TCP ("6"), UDP ("17"), and ICMPv6 ("58").
	Protocol *string `json:"protocol,omitempty"`

	// Type of destination for the rule. The default is `CIDR_BLOCK`.
	// Allowed values:
	//   * `CIDR_BLOCK`: If the rule's `destination` is an IP address range in CIDR notation.
	//   * `SERVICE_CIDR_BLOCK`: If the rule's `destination` is the `cidrBlock` value for a
	//     Service (the rule is for traffic destined for a
	//     particular `Service` through a service gateway).
	DestinationType EgressSecurityRuleDestinationTypeEnum `json:"destinationType,omitempty"`

	IcmpOptions *IcmpOptions `json:"icmpOptions,omitempty"`

	// A stateless rule allows traffic in one direction. Remember to add a corresponding
	// stateless rule in the other direction if you need to support bidirectional traffic. For
	// example, if egress traffic allows TCP destination port 80, there should be an ingress
	// rule to allow TCP source port 80. Defaults to false, which means the rule is stateful
	// and a corresponding rule is not necessary for bidirectional traffic.
	IsStateless *bool `json:"isStateless,omitempty"`

	TcpOptions *TcpOptions `json:"tcpOptions,omitempty"`

	UdpOptions *UdpOptions `json:"udpOptions,omitempty"`

	// An optional description of your choice for the rule.
	Description *string `json:"description,omitempty"`
}

// IngressSecurityRule A rule for allowing inbound IP packets.
type IngressSecurityRule struct {

	// The transport protocol. Specify either `all` or an IPv4 protocol number as
	// defined in
	// Protocol Numbers (http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml).
	// Options are supported only for ICMP ("1"), TCP ("6"), UDP ("17"), and ICMPv6 ("58").
	Protocol *string `json:"protocol,omitempty"`

	// Conceptually, this is the range of IP addresses that a packet coming into the instance
	// can come from.
	// Allowed values:
	//   * IP address range in CIDR notation. For example: `192.168.1.0/24` or `2001:0db8:0123:45::/56`.
	//     IPv6 addressing is supported for all commercial and government regions. See
	//     IPv6 Addresses (https://docs.cloud.oracle.com/iaas/Content/Network/Concepts/ipv6.htm).
	//   * The `cidrBlock` value for a Service, if you're
	//     setting up a security list rule for traffic coming from a particular `Service` through
	//     a service gateway. For example: `oci-phx-objectstorage`.
	Source *string `json:"source,omitempty"`

	IcmpOptions *IcmpOptions `json:"icmpOptions,omitempty"`

	// A stateless rule allows traffic in one direction. Remember to add a corresponding
	// stateless rule in the other direction if you need to support bidirectional traffic. For
	// example, if ingress traffic allows TCP destination port 80, there should be an egress
	// rule to allow TCP source port 80. Defaults to false, which means the rule is stateful
	// and a corresponding rule is not necessary for bidirectional traffic.
	IsStateless *bool `json:"isStateless,omitempty"`

	// Type of source for the rule. The default is `CIDR_BLOCK`.
	//   * `CIDR_BLOCK`: If the rule's `source` is an IP address range in CIDR notation.
	//   * `SERVICE_CIDR_BLOCK`: If the rule's `source` is the `cidrBlock` value for a
	//     Service (the rule is for traffic coming from a
	//     particular `Service` through a service gateway).
	SourceType IngressSecurityRuleSourceTypeEnum `json:"sourceType,omitempty"`

	TcpOptions *TcpOptions `json:"tcpOptions,omitempty"`

	UdpOptions *UdpOptions `json:"udpOptions,omitempty"`

	// An optional description of your choice for the rule.
	Description *string `json:"description,omitempty"`
}

// IngressSecurityRuleForNSG is IngressSecurityRule for NSG
type IngressSecurityRuleForNSG struct {
	//IngressSecurityRule ID for NSG.
	// +optional
	// Deprecated: this field is not populated and used during reconciliation
	ID                  *string `json:"id,omitempty"`
	IngressSecurityRule `json:"ingressRule,omitempty"`
}

// EgressSecurityRuleForNSG is EgressSecurityRule for NSG.
type EgressSecurityRuleForNSG struct {
	// EgressSecurityRule ID for NSG.
	// +optional
	// Deprecated: this field is not populated and used during reconciliation
	ID                 *string `json:"id,omitempty"`
	EgressSecurityRule `json:"egressRule,omitempty"`
}

// IngressSecurityRuleSourceTypeEnum Enum with underlying type: string.
type IngressSecurityRuleSourceTypeEnum string

// Set of constants representing the allowable values for IngressSecurityRuleSourceTypeEnum
const (
	IngressSecurityRuleSourceTypeCidrBlock        IngressSecurityRuleSourceTypeEnum = "CIDR_BLOCK"
	IngressSecurityRuleSourceTypeServiceCidrBlock IngressSecurityRuleSourceTypeEnum = "SERVICE_CIDR_BLOCK"
)

// UdpOptions Optional and valid only for UDP. Use to specify particular destination ports for UDP rules.
// If you specify UDP as the protocol but omit this object, then all destination ports are allowed.
type UdpOptions struct {
	DestinationPortRange *PortRange `json:"destinationPortRange,omitempty"`

	SourcePortRange *PortRange `json:"sourcePortRange,omitempty"`
}

// IcmpOptions Optional and valid only for ICMP and ICMPv6. Use to specify a particular ICMP type and code
// as defined in:
// - ICMP Parameters (http://www.iana.org/assignments/icmp-parameters/icmp-parameters.xhtml)
// - ICMPv6 Parameters (https://www.iana.org/assignments/icmpv6-parameters/icmpv6-parameters.xhtml)
// If you specify ICMP or ICMPv6 as the protocol but omit this object, then all ICMP types and
// codes are allowed. If you do provide this object, the type is required and the code is optional.
// To enable MTU negotiation for ingress internet traffic via IPv4, make sure to allow type 3 ("Destination
// Unreachable") code 4 ("Fragmentation Needed and Don't Fragment was Set"). If you need to specify
// multiple codes for a single type, create a separate security list rule for each.
type IcmpOptions struct {

	// The ICMP type.
	Type *int `json:"type,omitempty"`

	// The ICMP code (optional).
	Code *int `json:"code,omitempty"`
}

// TcpOptions Optional and valid only for TCP. Use to specify particular destination ports for TCP rules.
// If you specify TCP as the protocol but omit this object, then all destination ports are allowed.
type TcpOptions struct {
	DestinationPortRange *PortRange `json:"destinationPortRange,omitempty"`

	SourcePortRange *PortRange `json:"sourcePortRange,omitempty"`
}

// PortRange The representation of PortRange.
type PortRange struct {

	// The maximum port number, which must not be less than the minimum port number. To specify
	// a single port number, set both the min and max to the same value.
	Max *int `json:"max,omitempty"`

	// The minimum port number, which must not be greater than the maximum port number.
	Min *int `json:"min,omitempty"`
}

const (
	// EgressSecurityRuleDestinationTypeCidrBlock is the contant for CIDR block security rule destination type
	EgressSecurityRuleDestinationTypeCidrBlock EgressSecurityRuleDestinationTypeEnum = "CIDR_BLOCK"
)

type EgressSecurityRuleDestinationTypeEnum string

// SecurityList defines the configureation for the security list for network virtual firewall
// https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/securitylists.htm
type SecurityList struct {
	// ID of the SecurityList.
	// +optional
	ID *string `json:"id,omitempty"`
	// SecurityList Name.
	// +optional
	Name string `json:"name"`
	// EgressRules on the SecurityList.
	// +optional
	EgressRules []EgressSecurityRule `json:"egressRules,omitempty"`
	//IngressRules on the SecurityList.
	// +optional
	IngressRules []IngressSecurityRule `json:"ingressRules,omitempty"`
}

// Role defines the unique role of a subnet.
type Role string

type SubnetType string

// Subnet defines the configuration for a network's subnet
// https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/managingVCNs_topic-Overview_of_VCNs_and_Subnets.htm#Overview
type Subnet struct {
	// Role defines the subnet role (eg. control-plane, control-plane-endpoint, service-lb, worker).
	Role Role `json:"role"`
	// Subnet OCID.
	// +optional
	ID *string `json:"id,omitempty"`
	// Subnet Name.
	Name string `json:"name"`
	// Subnet CIDR.
	// +optional
	CIDR string `json:"cidr,omitempty"`
	// Type defines the subnet type (e.g. public, private).
	// +optional
	Type SubnetType `json:"type,omitempty"`
	// The security list associated with Subnet.
	// +optional
	SecurityList *SecurityList `json:"securityList,omitempty"`
}

// NSG defines configuration for a Network Security Group.
// https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/networksecuritygroups.htm
type NSG struct {
	// NSG OCID.
	// +optional
	ID *string `json:"id,omitempty"`
	// NSG Name.
	Name string `json:"name"`
	// Role defines the NSG role (eg. control-plane, control-plane-endpoint, service-lb, worker).
	Role Role `json:"role,omitempty"`
	// EgressRules on the NSG.
	// +optional
	EgressRules []EgressSecurityRuleForNSG `json:"egressRules,omitempty"`
	// IngressRules on the NSG.
	// +optional
	IngressRules []IngressSecurityRuleForNSG `json:"ingressRules,omitempty"`
}

// VCN dfines the configuration for a Virtual Cloud Network.
// https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/overview.htm
type VCN struct {
	// VCN OCID.
	// +optional
	ID *string `json:"id,omitempty"`
	// VCN Name.
	// +optional
	Name string `json:"name"`
	// VCN CIDR.
	// +optional
	CIDR string `json:"cidr,omitempty"`

	// ID of Nat Gateway.
	// +optional
	NatGatewayId *string `json:"natGatewayId,omitempty"`

	// ID of Internet Gateway.
	// +optional
	InternetGatewayId *string `json:"internetGatewayId,omitempty"`

	// ID of Service Gateway.
	// +optional
	ServiceGatewayId *string `json:"serviceGatewayId,omitempty"`

	// ID of Private Route Table.
	// +optional
	PrivateRouteTableId *string `json:"privateRouteTableId,omitempty"`

	// ID of Public Route Table.
	// +optional
	PublicRouteTableId *string `json:"publicRouteTableId,omitempty"`

	// Subnets is the configuration for subnets required in the VCN.
	// +optional
	// +listType=map
	// +listMapKey=name
	Subnets []*Subnet `json:"subnets,omitempty"`

	// NetworkSecurityGroups is the configuration for the Network Security Groups required in the VCN.
	// +optional
	// +listType=map
	// +listMapKey=name
	NetworkSecurityGroups []*NSG `json:"networkSecurityGroups,omitempty"`
}

//LoadBalancer Configuration
type LoadBalancer struct {
	//LoadBalancer Name.
	// +optional
	Name string `json:"name"`

	// ID of Load Balancer.
	// +optional
	LoadBalancerId *string `json:"loadBalancerId,omitempty"`
}

// NetworkSpec specifies what the OCI networking resources should look like.
type NetworkSpec struct {
	// SkipNetworkManagement defines if the networking spec(VCN related) specified by the user needs to be reconciled(actioned-upon)
	// or used as it is. APIServerLB will still be reconciled.
	// +optional
	SkipNetworkManagement bool `json:"skipNetworkManagement,omitempty"`

	// VCN configuration.
	// +optional
	Vcn VCN `json:"vcn,omitempty"`

	//API Server LB configuration.
	// +optional
	APIServerLB LoadBalancer `json:"apiServerLoadBalancer,omitempty"`

	// VCNPeering configuration.
	// +optional
	VCNPeering *VCNPeering `json:"vcnPeering,omitempty"`
}

// VCNPeering defines the VCN peering details of the workload cluster VCN.
type VCNPeering struct {

	// DRG configuration refers to the DRG which has to be created if required. If management cluster
	// and workload cluster shares the same DRG, this fields is not required to be specified.
	// +optional
	DRG *DRG `json:"drg,omitempty"`

	// PeerRouteRules defines the routing rules which will be added to the private route tables
	// of the workload cluster VCN. The routes defined here will be directed to DRG.
	PeerRouteRules []PeerRouteRule `json:"peerRouteRules,omitempty"`

	// RemotePeeringConnections defines the RPC connections which be established with the
	// workload cluster DRG.
	RemotePeeringConnections []RemotePeeringConnection `json:"remotePeeringConnections,omitempty"`
}

// DRG defines the configuration for a Dynamic Resource Group.
type DRG struct {

	// Manage defines whether the DRG has to be managed(including create). If set to false(the default) the ID
	// has to be specified by the user to a valid DRG ID to which the VCN has to be attached.
	// +optional
	Manage bool `json:"manage,omitempty"`

	// Name is the name of the created DRG.
	// +optional
	Name string `json:"name,omitempty"`

	// ID is the OCID for the created DRG.
	// +optional
	ID *string `json:"id,omitempty"`

	// VcnAttachmentId is the ID of the VCN attachment of the DRG.
	// The workload cluster VCN can be attached to either the management cluster VCN if they are sharing the same DRG
	// or to the workload cluster DRG.
	// +optional
	VcnAttachmentId *string `json:"vcnAttachmentId,omitempty"`
}

// PeerRouteRule defines a Route Rule to be routed via a DRG.
type PeerRouteRule struct {
	// VCNCIDRRange is the CIDR Range of peer VCN to which the
	// workload cluster VCN will be peered. The CIDR range is required to add the route rule
	// in the workload cluster VCN, the route rule will forward any traffic to the CIDR to the DRG.
	// +optional
	VCNCIDRRange string `json:"vcnCIDRRange,omitempty"`
}

// RemotePeeringConnection is used to peer VCNs residing in different regions(typically).
// Remote VCN Peering is explained here - https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/remoteVCNpeering.htm
type RemotePeeringConnection struct {

	// ManagePeerRPC will define if the Peer VCN needs to be managed. If set to true
	// a Remote Peering Connection will be created in the Peer DRG and the connection
	// will be created between local and peer RPC.
	ManagePeerRPC bool `json:"managePeerRPC,omitempty"`

	// PeerRegionName defined the region name of Peer VCN.
	PeerRegionName string `json:"peerRegionName,omitempty"`

	// PeerDRGId defines the DRG ID of the peer.
	PeerDRGId *string `json:"peerDRGId,omitempty"`

	// PeerRPCConnectionId defines the RPC ID of peer. If ManagePeerRPC is set to true
	// this will be created by Cluster API Provider for OCI, otherwise this has be defined by the
	// user.
	PeerRPCConnectionId *string `json:"peerRPCConnectionId,omitempty"`

	// RPCConnectionId is the connection ID of the connection between peer and local RPC.
	RPCConnectionId *string `json:"rpcConnectionId,omitempty"`
}
