/*
 *
 * Copyright (c) 2022, Oracle and/or its affiliates.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package v1beta2

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var clusterlogger = ctrl.Log.WithName("ocicluster-resource")

type OCIClusterWebhook struct{}

var (
	_ webhook.CustomDefaulter = &OCIClusterWebhook{}
	_ webhook.CustomValidator = &OCIClusterWebhook{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ocicluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ociclusters,versions=v1beta2,name=validation.ocicluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ocicluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ociclusters,versions=v1beta2,name=default.ocicluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

func (*OCIClusterWebhook) Default(_ context.Context, obj runtime.Object) error {
	c, ok := obj.(*OCICluster)
	if !ok {
		return fmt.Errorf("expected an OCICluster object but got %T", c)
	}

	if c.Spec.OCIResourceIdentifier == "" {
		c.Spec.OCIResourceIdentifier = string(uuid.NewUUID())
	}
	if !c.Spec.NetworkSpec.SkipNetworkManagement {
		c.Spec.NetworkSpec.Vcn.Subnets = c.SubnetSpec()
		c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List = c.NSGSpec()
	}

	return nil
}

func (c *OCICluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(OCIClusterWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*OCIClusterWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*OCICluster)
	if !ok {
		return nil, fmt.Errorf("expected an OCICluster object but got %T", c)
	}

	clusterlogger.Info("validate update cluster", "name", c.Name)

	var allErrs field.ErrorList
	var ipv6hextets []*string
	var hextatassigned bool

	if c.GetControlPlaneEndpointSubnet() != nil && c.GetControlPlaneMachineSubnet() != nil && c.GetServiceLoadBalancerSubnet() != nil {
		ipv6hextets = append(ipv6hextets,
			c.GetControlPlaneEndpointSubnet().Ipv6CidrBlockHextet,
			c.GetControlPlaneMachineSubnet().Ipv6CidrBlockHextet,
			c.GetServiceLoadBalancerSubnet().Ipv6CidrBlockHextet)

		nodeSubnets := c.GetNodeSubnet()
		if len(nodeSubnets) > 0 {
			ipv6hextets = append(ipv6hextets, nodeSubnets[0].Ipv6CidrBlockHextet)
		}
	}

	for _, hextet := range ipv6hextets {
		if hextet != nil {
			hextatassigned = true
			break
		}
	}

	if hextatassigned {
		if c.Spec.NetworkSpec.Vcn.IsIpv6Enabled == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.IsIpv6Enabled"), c.Spec.NetworkSpec.Vcn.IsIpv6Enabled, "field needs to be true and not nil"))
		}
		if c.Spec.NetworkSpec.Vcn.IsIpv6Enabled == common.Bool(false) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.IsIpv6Enabled"), c.Spec.NetworkSpec.Vcn.IsIpv6Enabled, "field needs to be true"))
		}
	}

	// If Skip field is true, ID field of VCN should be specified
	if c.Spec.NetworkSpec.Vcn.Skip == *common.Bool(true) {
		if c.Spec.NetworkSpec.Vcn.ID == common.String("") || c.Spec.NetworkSpec.Vcn.ID == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.ID"), c.Spec.NetworkSpec.Vcn.ID, "field is required"))
		}

		// If Skip field is True, Skip field of InternetGateway should be true
		if c.Spec.NetworkSpec.Vcn.InternetGateway.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.InternetGateway.Skip"), c.Spec.NetworkSpec.Vcn.InternetGateway.Skip, "field requires to be true when VCN is skipped"))
		}

		// If Skip field is True, Skip field of ServiceGateway should be true
		if c.Spec.NetworkSpec.Vcn.ServiceGateway.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.ServiceGateway.Skip"), c.Spec.NetworkSpec.Vcn.ServiceGateway.Skip, "field requires to be true when VCN is skipped"))
		}

		// If Skip field is True, Skip field of NATGateway should be true
		if c.Spec.NetworkSpec.Vcn.NATGateway.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.NATGateway.Skip"), c.Spec.NetworkSpec.Vcn.NATGateway.Skip, "field requires to be true when VCN is skipped"))
		}

		// If Skip field is True, Skip field of RouteTable should be true
		if c.Spec.NetworkSpec.Vcn.RouteTable.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.RouteTable.Skip"), c.Spec.NetworkSpec.Vcn.RouteTable.Skip, "field requires to be true when VCN is skipped"))
		}

		// For each subnet
		for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {

			// if Skip field is true, ID field of Subnet should also be specified
			if subnet.Skip == *common.Bool(true) {
				if subnet.ID == common.String("") || subnet.ID == nil {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "subnet.ID"), subnet.ID, "field is required"))
				}
			}
			// if ID field is specified, Skip field of Subnet should also be true
			if subnet.ID != common.String("") {
				if subnet.Skip != *common.Bool(true) {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "subnet.Skip"), subnet.Skip, "field requires to be true if Subnet ID is specified"))
				}
			}
		}
	} else {
		// If Skip field of VCN is false, for each subnet in that VCN the Skip field of Subnet cannot be true
		for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {
			if subnet.Skip == *common.Bool(true) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "subnet.Skip"), subnet.Skip, "field cannot be true when VCN is not skipped"))
			}
		}
	}

	allErrs = append(allErrs, c.validate(nil)...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*OCIClusterWebhook) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*OCICluster)
	if !ok {
		return nil, fmt.Errorf("expected an OCICluster object but got %T", c)
	}
	clusterlogger.Info("validate delete cluster", "name", c.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*OCIClusterWebhook) ValidateUpdate(_ context.Context, oldRaw, newObj runtime.Object) (admission.Warnings, error) {
	c, ok := newObj.(*OCICluster)
	if !ok {
		return nil, fmt.Errorf("expected an OCICluster object but got %T", c)
	}

	clusterlogger.Info("validate update cluster", "name", c.Name)

	var allErrs field.ErrorList

	oldCluster, ok := oldRaw.(*OCICluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected an OCICluster but got a %T", oldRaw))
	}

	if c.Spec.Region != oldCluster.Spec.Region {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "region"), c.Spec.Region, "field is immutable"))
	}

	if c.Spec.OCIResourceIdentifier != oldCluster.Spec.OCIResourceIdentifier {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "ociResourceIdentifier"), c.Spec.OCIResourceIdentifier, "field is immutable"))
	}

	if c.Spec.CompartmentId != oldCluster.Spec.CompartmentId {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "compartmentId"), c.Spec.CompartmentId, "field is immutable"))
	}

	// If Skip field is true, ID field of VCN should be specified
	if c.Spec.NetworkSpec.Vcn.Skip == *common.Bool(true) {
		if c.Spec.NetworkSpec.Vcn.ID == common.String("") || c.Spec.NetworkSpec.Vcn.ID == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.ID"), c.Spec.NetworkSpec.Vcn.ID, "field is required"))
		}

		// If Skip field is True, Skip field of InternetGateway should be true
		if c.Spec.NetworkSpec.Vcn.InternetGateway.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.InternetGateway.Skip"), c.Spec.NetworkSpec.Vcn.InternetGateway.Skip, "field requires to be true when VCN is skipped"))
		}

		// If Skip field is True, Skip field of ServiceGateway should be true
		if c.Spec.NetworkSpec.Vcn.ServiceGateway.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.ServiceGateway.Skip"), c.Spec.NetworkSpec.Vcn.ServiceGateway.Skip, "field requires to be true when VCN is skipped"))
		}

		// If Skip field is True, Skip field of NATGateway should be true
		if c.Spec.NetworkSpec.Vcn.NATGateway.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.NATGateway.Skip"), c.Spec.NetworkSpec.Vcn.NATGateway.Skip, "field requires to be true when VCN is skipped"))
		}

		// If Skip field is True, Skip field of RouteTable should be true
		if c.Spec.NetworkSpec.Vcn.RouteTable.Skip != *common.Bool(true) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "NetworkSpec.Vcn.RouteTable.Skip"), c.Spec.NetworkSpec.Vcn.RouteTable.Skip, "field requires to be true when VCN is skipped"))
		}

		// For each subnet
		for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {

			// if Skip field is true, ID field of Subnet should also be specified
			if subnet.Skip == *common.Bool(true) {
				if subnet.ID == common.String("") || subnet.ID == nil {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "subnet.ID"), subnet.ID, "field is required"))
				}
			}
			// if ID field is specified, Skip field of Subnet should also be true
			if subnet.ID != common.String("") {
				if subnet.Skip != *common.Bool(true) {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "subnet.Skip"), subnet.Skip, "field requires to be true if Subnet ID is specified"))
				}
			}
		}
	} else {
		// If Skip field of VCN is false, for each subnet in that VCN the Skip field of Subnet cannot be true
		for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {
			if subnet.Skip == *common.Bool(true) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "subnet.Skip"), subnet.Skip, "field cannot be true when VCN is not skipped"))
			}
		}
	}

	allErrs = append(allErrs, c.validate(oldCluster)...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

func (c *OCICluster) validate(old *OCICluster) field.ErrorList {
	var allErrs field.ErrorList

	var oldNetworkSpec NetworkSpec
	if old != nil {
		oldNetworkSpec = old.Spec.NetworkSpec
	}

	allErrs = append(allErrs, ValidateNetworkSpec(OCIClusterSubnetRoles, c.Spec.NetworkSpec, oldNetworkSpec, field.NewPath("spec").Child("networkSpec"))...)
	allErrs = append(allErrs, ValidateClusterName(c.Name)...)

	if len(c.Spec.CompartmentId) <= 0 {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "compartmentId"), c.Spec.CompartmentId, "field is required"))
	}

	// Handle case where CompartmentId exists, but isn't valid
	// the separate "blank" check above is a more clear error for the user
	if len(c.Spec.CompartmentId) > 0 && !ValidOcid(c.Spec.CompartmentId) {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "compartmentId"), c.Spec.CompartmentId, "field is invalid"))
	}

	if len(c.Spec.OCIResourceIdentifier) <= 0 {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "ociResourceIdentifier"), c.Spec.OCIResourceIdentifier, "field is required"))
	}

	if !ValidRegion(c.Spec.Region) {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "region"), c.Spec.Region, "field is invalid. See https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm"))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return allErrs
}

func (c *OCICluster) SubnetSpec() []*Subnet {
	subnets := c.Spec.NetworkSpec.Vcn.Subnets

	cpEndpointSubnet := c.GetControlPlaneEndpointSubnet()
	if cpEndpointSubnet == nil {
		subnets = append(subnets, &Subnet{
			Role: ControlPlaneEndpointRole,
			Name: ControlPlaneEndpointDefaultName,
			CIDR: ControlPlaneEndpointSubnetDefaultCIDR,
			Type: Public,
		})
	} else {
		if cpEndpointSubnet.CIDR == "" {
			cpEndpointSubnet.CIDR = ControlPlaneEndpointSubnetDefaultCIDR
		}
	}
	cpSubnet := c.GetControlPlaneMachineSubnet()
	if cpSubnet == nil {
		subnets = append(subnets, &Subnet{
			Role: ControlPlaneRole,
			Name: ControlPlaneDefaultName,
			CIDR: ControlPlaneMachineSubnetDefaultCIDR,
			Type: Private,
		})
	} else {
		if cpSubnet.CIDR == "" {
			cpSubnet.CIDR = ControlPlaneMachineSubnetDefaultCIDR
		}
	}
	lbServiceSubnet := c.GetServiceLoadBalancerSubnet()
	if lbServiceSubnet == nil {
		subnets = append(subnets, &Subnet{
			Role: ServiceLoadBalancerRole,
			Name: ServiceLBDefaultName,
			CIDR: ServiceLoadBalancerDefaultCIDR,
			Type: Public,
		})
	} else {
		if lbServiceSubnet.CIDR == "" {
			lbServiceSubnet.CIDR = ServiceLoadBalancerDefaultCIDR
		}
	}
	nodeSubnet := c.GetNodeSubnet()
	if nodeSubnet == nil {
		subnets = append(subnets, &Subnet{
			Role: WorkerRole,
			Name: WorkerDefaultName,
			CIDR: WorkerSubnetDefaultCIDR,
			Type: Private,
		})
	} else {
		for _, subnet := range nodeSubnet {
			if subnet.CIDR == "" {
				subnet.CIDR = WorkerSubnetDefaultCIDR
			}
		}
	}
	return subnets
}

func (c *OCICluster) NSGSpec() []*NSG {
	nsgs := c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List
	if c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.Skip {
		return nsgs
	}
	if !c.IsNSGExitsByRole(ControlPlaneEndpointRole) && !c.IsSecurityListExitsByRole(ControlPlaneEndpointRole) {
		nsgs = append(nsgs, &NSG{
			Role:         ControlPlaneEndpointRole,
			Name:         ControlPlaneEndpointDefaultName,
			IngressRules: c.GetControlPlaneEndpointDefaultIngressRules(),
			EgressRules:  c.GetControlPlaneEndpointDefaultEgressRules(),
		})
	}
	if !c.IsNSGExitsByRole(ControlPlaneRole) && !c.IsSecurityListExitsByRole(ControlPlaneRole) {
		nsgs = append(nsgs, &NSG{
			Role:         ControlPlaneRole,
			Name:         ControlPlaneDefaultName,
			IngressRules: c.GetControlPlaneMachineDefaultIngressRules(),
			EgressRules:  c.GetControlPlaneMachineDefaultEgressRules(),
		})
	}
	if !c.IsNSGExitsByRole(WorkerRole) && !c.IsSecurityListExitsByRole(WorkerRole) {
		nsgs = append(nsgs, &NSG{
			Role:         WorkerRole,
			Name:         WorkerDefaultName,
			IngressRules: c.GetNodeDefaultIngressRules(),
			EgressRules:  c.GetNodeDefaultEgressRules(),
		})
	}
	if !c.IsNSGExitsByRole(ServiceLoadBalancerRole) && !c.IsSecurityListExitsByRole(ServiceLoadBalancerRole) {
		nsgs = append(nsgs, &NSG{
			Role:         ServiceLoadBalancerRole,
			Name:         ServiceLBDefaultName,
			IngressRules: c.GetServiceLoadBalancerDefaultIngressRules(),
			EgressRules:  c.GetServiceLoadBalancerDefaultEgressRules(),
		})
	}
	return nsgs
}

func (c *OCICluster) GetControlPlaneMachineDefaultIngressRules() []IngressSecurityRuleForNSG {
	return []IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Kubernetes API endpoint to Control Plane(apiserver port) communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Control plane node to Control Plane(apiserver port) communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Worker Node to Control Plane(apiserver port) communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("etcd client communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(2379),
						Min: common.Int(2379),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("etcd peer"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(2380),
						Min: common.Int(2380),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Inbound SSH traffic to Control Plane"),
				Protocol:    common.String("6"),
				SourceType:  IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String("0.0.0.0/0"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(22),
						Min: common.Int(22),
					},
				},
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Control Plane to Control Plane Kubelet Communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
	}
}

func (c *OCICluster) GetControlPlaneMachineDefaultEgressRules() []EgressSecurityRuleForNSG {
	return []EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: EgressSecurityRule{
				Description:     common.String("Control Plane access to Internet"),
				Protocol:        common.String("all"),
				DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (c *OCICluster) GetNodeDefaultIngressRules() []IngressSecurityRuleForNSG {
	return []IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Inbound SSH traffic to worker node"),
				Protocol:    common.String("6"),
				SourceType:  IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String("0.0.0.0/0"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(22),
						Min: common.Int(22),
					},
				},
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Control Plane to worker node Kubelet Communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Worker node to worker node Kubelet Communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking (BGP)"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(179),
						Min: common.Int(179),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Calico networking with IP-in-IP enabled"),
				Protocol:    common.String("4"),
				SourceType:  IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Worker node to default NodePort ingress communication"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(32767),
						Min: common.Int(30000),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(WorkerSubnetDefaultCIDR),
			},
		},
	}
}

func (c *OCICluster) GetNodeDefaultEgressRules() []EgressSecurityRuleForNSG {
	return []EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: EgressSecurityRule{
				Description:     common.String("Worker node access to Internet"),
				Protocol:        common.String("all"),
				DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (c *OCICluster) GetServiceLoadBalancerDefaultIngressRules() []IngressSecurityRuleForNSG {
	return []IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Accept http traffic on port 80"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(80),
						Min: common.Int(80),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Accept https traffic on port 443"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(443),
						Min: common.Int(443),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (c *OCICluster) GetServiceLoadBalancerDefaultEgressRules() []EgressSecurityRuleForNSG {
	return []EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: EgressSecurityRule{
				Destination:     common.String(WorkerSubnetDefaultCIDR),
				Protocol:        common.String("6"),
				DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(32767),
						Min: common.Int(30000),
					},
				},
				Description: common.String("Service LoadBalancer to default NodePort egress communication"),
			},
		},
	}
}

func (c *OCICluster) GetControlPlaneEndpointDefaultIngressRules() []IngressSecurityRuleForNSG {
	return []IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("External access to Kubernetes API endpoint"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
		{
			IngressSecurityRule: IngressSecurityRule{
				Description: common.String("Path discovery"),
				Protocol:    common.String("1"),
				IcmpOptions: &IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(VcnDefaultCidr),
			},
		},
	}
}

func (c *OCICluster) GetControlPlaneEndpointDefaultEgressRules() []EgressSecurityRuleForNSG {
	return []EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: EgressSecurityRule{
				Description: common.String("Kubernetes API traffic to Control Plane"),
				Protocol:    common.String("6"),
				TcpOptions: &TcpOptions{
					DestinationPortRange: &PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				DestinationType: EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(ControlPlaneMachineSubnetDefaultCIDR),
			},
		},
	}
}

func (c *OCICluster) GetControlPlaneEndpointSubnet() *Subnet {
	for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == ControlPlaneEndpointRole {
			return subnet
		}
	}
	return nil
}

func (c *OCICluster) GetControlPlaneMachineSubnet() *Subnet {
	for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == ControlPlaneRole {
			return subnet
		}
	}
	return nil
}

func (c *OCICluster) GetServiceLoadBalancerSubnet() *Subnet {
	for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == ServiceLoadBalancerRole {
			return subnet
		}
	}
	return nil
}

func (c *OCICluster) GetNodeSubnet() []*Subnet {
	var nodeSubnets []*Subnet
	for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {
		if subnet.Role == WorkerRole {
			nodeSubnets = append(nodeSubnets, subnet)
		}
	}
	return nodeSubnets
}

func (c *OCICluster) IsNSGExitsByRole(role Role) bool {
	for _, nsg := range c.Spec.NetworkSpec.Vcn.NetworkSecurityGroup.List {
		if role == nsg.Role {
			return true
		}
	}
	return false
}

func (c *OCICluster) IsSecurityListExitsByRole(role Role) bool {
	for _, subnet := range c.Spec.NetworkSpec.Vcn.Subnets {
		if role == subnet.Role {
			if subnet.SecurityList != nil {
				return true
			}
		}
	}
	return false
}
