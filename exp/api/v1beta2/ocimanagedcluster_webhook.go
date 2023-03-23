/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	"fmt"
	"reflect"

	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	"github.com/oracle/oci-go-sdk/v65/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var managedclusterlogger = ctrl.Log.WithName("ocimanagedcluster-resource")

var (
	_ webhook.Defaulter = &OCIManagedCluster{}
	_ webhook.Validator = &OCIManagedCluster{}
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-ocimanagedcluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedclusters,versions=v1beta1,name=validation.ocimanagedcluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-ocimanagedcluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=ocimanagedclusters,versions=v1beta1,name=default.ocimanagedcluster.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

func (c *OCIManagedCluster) Default() {
	if c.Spec.OCIResourceIdentifier == "" {
		c.Spec.OCIResourceIdentifier = string(uuid.NewUUID())
	}
	if !c.Spec.NetworkSpec.SkipNetworkManagement {
		if len(c.Spec.NetworkSpec.Vcn.Subnets) == 0 {
			subnets := make([]*infrastructurev1beta2.Subnet, 4)
			subnets[0] = &infrastructurev1beta2.Subnet{
				Role: infrastructurev1beta2.ControlPlaneEndpointRole,
				Name: infrastructurev1beta2.ControlPlaneEndpointDefaultName,
				CIDR: infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR,
				Type: infrastructurev1beta2.Public,
			}

			subnets[1] = &infrastructurev1beta2.Subnet{
				Role: infrastructurev1beta2.ServiceLoadBalancerRole,
				Name: infrastructurev1beta2.ServiceLBDefaultName,
				CIDR: infrastructurev1beta2.ServiceLoadBalancerDefaultCIDR,
				Type: infrastructurev1beta2.Public,
			}
			subnets[2] = &infrastructurev1beta2.Subnet{
				Role: infrastructurev1beta2.WorkerRole,
				Name: infrastructurev1beta2.WorkerDefaultName,
				CIDR: infrastructurev1beta2.WorkerSubnetDefaultCIDR,
				Type: infrastructurev1beta2.Private,
			}
			subnets[3] = &infrastructurev1beta2.Subnet{
				Role: infrastructurev1beta2.PodRole,
				Name: PodDefaultName,
				CIDR: PodDefaultCIDR,
				Type: infrastructurev1beta2.Private,
			}
			c.Spec.NetworkSpec.Vcn.Subnets = subnets
		}
		if len(c.Spec.NetworkSpec.Vcn.NetworkSecurityGroups.NSGList) == 0 {
			nsgs := make([]*infrastructurev1beta2.NSG, 4)
			nsgs[0] = &infrastructurev1beta2.NSG{
				Role:         infrastructurev1beta2.ControlPlaneEndpointRole,
				Name:         infrastructurev1beta2.ControlPlaneEndpointDefaultName,
				IngressRules: c.GetControlPlaneEndpointDefaultIngressRules(),
				EgressRules:  c.GetControlPlaneEndpointDefaultEgressRules(),
			}
			nsgs[1] = &infrastructurev1beta2.NSG{
				Role:         infrastructurev1beta2.WorkerRole,
				Name:         infrastructurev1beta2.WorkerDefaultName,
				IngressRules: c.GetWorkerDefaultIngressRules(),
				EgressRules:  c.GetWorkerDefaultEgressRules(),
			}
			nsgs[2] = &infrastructurev1beta2.NSG{
				Role:         infrastructurev1beta2.ServiceLoadBalancerRole,
				Name:         infrastructurev1beta2.ServiceLBDefaultName,
				IngressRules: c.GetLBServiceDefaultIngressRules(),
				EgressRules:  c.GetLBServiceDefaultEgressRules(),
			}
			nsgs[3] = &infrastructurev1beta2.NSG{
				Role:         infrastructurev1beta2.PodRole,
				Name:         PodDefaultName,
				IngressRules: c.GetPodDefaultIngressRules(),
				EgressRules:  c.GetPodDefaultEgressRules(),
			}
			c.Spec.NetworkSpec.Vcn.NetworkSecurityGroups.NSGList = nsgs
		}
		if c.Spec.NetworkSpec.Vcn.CIDR == "" {
			c.Spec.NetworkSpec.Vcn.CIDR = infrastructurev1beta2.VcnDefaultCidr
		}
	}
}

func (c *OCIManagedCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *OCIManagedCluster) ValidateCreate() error {
	managedclusterlogger.Info("validate create cluster", "name", c.Name)

	var allErrs field.ErrorList

	allErrs = append(allErrs, c.validate(nil)...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *OCIManagedCluster) ValidateDelete() error {
	managedclusterlogger.Info("validate delete cluster", "name", c.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *OCIManagedCluster) ValidateUpdate(old runtime.Object) error {
	managedclusterlogger.Info("validate update cluster", "name", c.Name)

	var allErrs field.ErrorList

	oldCluster, ok := old.(*OCIManagedCluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an OCIManagedCluster but got a %T", old))
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

	allErrs = append(allErrs, c.validate(oldCluster)...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(c.GroupVersionKind().GroupKind(), c.Name, allErrs)
}

func (c *OCIManagedCluster) validate(old *OCIManagedCluster) field.ErrorList {
	var allErrs field.ErrorList

	var oldNetworkSpec infrastructurev1beta2.NetworkSpec
	if old != nil {
		oldNetworkSpec = old.Spec.NetworkSpec
	}

	allErrs = append(allErrs, infrastructurev1beta2.ValidateNetworkSpec(infrastructurev1beta2.OCIManagedClusterSubnetRoles, c.Spec.NetworkSpec, oldNetworkSpec, field.NewPath("spec").Child("networkSpec"))...)
	allErrs = append(allErrs, infrastructurev1beta2.ValidateClusterName(c.Name)...)

	if len(c.Spec.CompartmentId) <= 0 {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "compartmentId"), c.Spec.CompartmentId, "field is required"))
	}

	// Handle case where CompartmentId exists, but isn't valid
	// the separate "blank" check above is a more clear error for the user
	if len(c.Spec.CompartmentId) > 0 && !infrastructurev1beta2.ValidOcid(c.Spec.CompartmentId) {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "compartmentId"), c.Spec.CompartmentId, "field is invalid"))
	}

	if len(c.Spec.OCIResourceIdentifier) <= 0 {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "ociResourceIdentifier"), c.Spec.OCIResourceIdentifier, "field is required"))
	}

	if !infrastructurev1beta2.ValidRegion(c.Spec.Region) {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "region"), c.Spec.Region, "field is invalid. See https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm"))
	}

	if !reflect.DeepEqual(c.Spec.NetworkSpec.APIServerLB, infrastructurev1beta2.LoadBalancer{}) {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "networkSpec", "apiServerLoadBalancer"), c.Spec.NetworkSpec.APIServerLB, "cannot set loadbalancer in managed cluster"))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return allErrs
}

func (c *OCIManagedCluster) GetControlPlaneEndpointDefaultIngressRules() []infrastructurev1beta2.IngressSecurityRuleForNSG {
	return []infrastructurev1beta2.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Kubernetes worker to Kubernetes API endpoint communication."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(infrastructurev1beta2.WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Kubernetes worker to Kubernetes API endpoint communication."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(12250),
						Min: common.Int(12250),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(infrastructurev1beta2.WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Path Discovery."),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta2.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(infrastructurev1beta2.WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Pod to Kubernetes API endpoint communication (when using VCN-native pod networking)."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(PodDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Pod to Kubernetes API endpoint communication (when using VCN-native pod networking)."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(12250),
						Min: common.Int(12250),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(PodDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("External access to Kubernetes API endpoint."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (c *OCIManagedCluster) GetControlPlaneEndpointDefaultEgressRules() []infrastructurev1beta2.EgressSecurityRuleForNSG {
	return []infrastructurev1beta2.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description:     common.String("Allow Kubernetes API endpoint to communicate with OKE."),
				Protocol:        common.String("6"),
				DestinationType: infrastructurev1beta2.EgressSecurityRuleSourceTypeServiceCidrBlock,
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Path Discovery."),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta2.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleSourceTypeServiceCidrBlock,
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Allow Kubernetes API endpoint to communicate with worker nodes."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(infrastructurev1beta2.WorkerSubnetDefaultCIDR),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Path Discovery."),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta2.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(infrastructurev1beta2.WorkerSubnetDefaultCIDR),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description:     common.String("Allow Kubernetes API endpoint to communicate with pods (when using VCN-native pod networking)."),
				Protocol:        common.String("all"),
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(PodDefaultCIDR),
			},
		},
	}
}

func (c *OCIManagedCluster) GetWorkerDefaultIngressRules() []infrastructurev1beta2.IngressSecurityRuleForNSG {
	return []infrastructurev1beta2.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Allow Kubernetes API endpoint to communicate with worker nodes."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(10250),
						Min: common.Int(10250),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Path Discovery."),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta2.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Load Balancer to Worker nodes node ports."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(32767),
						Min: common.Int(30000),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String(infrastructurev1beta2.ServiceLoadBalancerDefaultCIDR),
			},
		},
	}
}

func (c *OCIManagedCluster) GetWorkerDefaultEgressRules() []infrastructurev1beta2.EgressSecurityRuleForNSG {
	return []infrastructurev1beta2.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description:     common.String("Allow worker nodes to communicate with OKE."),
				Protocol:        common.String("6"),
				DestinationType: infrastructurev1beta2.EgressSecurityRuleSourceTypeServiceCidrBlock,
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description:     common.String("Allow worker nodes to access pods."),
				Protocol:        common.String("all"),
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(PodDefaultCIDR),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Path Discovery."),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta2.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String("0.0.0.0/0"),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Kubernetes worker to Kubernetes API endpoint communication."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Kubernetes worker to Kubernetes API endpoint communication."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(12250),
						Min: common.Int(12250),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
	}
}

func (c *OCIManagedCluster) GetPodDefaultIngressRules() []infrastructurev1beta2.IngressSecurityRuleForNSG {
	return []infrastructurev1beta2.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Allow worker nodes to access pods."),
				Protocol:    common.String("all"),
				SourceType:  infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(infrastructurev1beta2.WorkerSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Allow Kubernetes API endpoint to communicate with pods."),
				Protocol:    common.String("all"),
				SourceType:  infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Allow pods to communicate with other pods."),
				Protocol:    common.String("all"),
				SourceType:  infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:      common.String(PodDefaultCIDR),
			},
		},
	}
}

func (c *OCIManagedCluster) GetPodDefaultEgressRules() []infrastructurev1beta2.EgressSecurityRuleForNSG {
	return []infrastructurev1beta2.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description:     common.String("Allow worker nodes to communicate with OCI Services."),
				Protocol:        common.String("6"),
				DestinationType: infrastructurev1beta2.EgressSecurityRuleSourceTypeServiceCidrBlock,
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Path Discovery."),
				Protocol:    common.String("1"),
				IcmpOptions: &infrastructurev1beta2.IcmpOptions{
					Type: common.Int(3),
					Code: common.Int(4),
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleSourceTypeServiceCidrBlock,
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description:     common.String("Allow pods to communicate with other pods."),
				Protocol:        common.String("all"),
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(PodDefaultCIDR),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Pod to Kubernetes API endpoint communication (when using VCN-native pod networking)."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(6443),
						Min: common.Int(6443),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Pod to Kubernetes API endpoint communication (when using VCN-native pod networking)."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(12250),
						Min: common.Int(12250),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(infrastructurev1beta2.ControlPlaneEndpointSubnetDefaultCIDR),
			},
		},
	}
}

func (c *OCIManagedCluster) GetLBServiceDefaultIngressRules() []infrastructurev1beta2.IngressSecurityRuleForNSG {
	return []infrastructurev1beta2.IngressSecurityRuleForNSG{
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Accept http traffic on port 80"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(80),
						Min: common.Int(80),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
		{
			IngressSecurityRule: infrastructurev1beta2.IngressSecurityRule{
				Description: common.String("Accept https traffic on port 443"),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(443),
						Min: common.Int(443),
					},
				},
				SourceType: infrastructurev1beta2.IngressSecurityRuleSourceTypeCidrBlock,
				Source:     common.String("0.0.0.0/0"),
			},
		},
	}
}

func (c *OCIManagedCluster) GetLBServiceDefaultEgressRules() []infrastructurev1beta2.EgressSecurityRuleForNSG {
	// TODO add service gateway rules
	return []infrastructurev1beta2.EgressSecurityRuleForNSG{
		{
			EgressSecurityRule: infrastructurev1beta2.EgressSecurityRule{
				Description: common.String("Load Balancer to Worker nodes node ports."),
				Protocol:    common.String("6"),
				TcpOptions: &infrastructurev1beta2.TcpOptions{
					DestinationPortRange: &infrastructurev1beta2.PortRange{
						Max: common.Int(32767),
						Min: common.Int(30000),
					},
				},
				DestinationType: infrastructurev1beta2.EgressSecurityRuleDestinationTypeCidrBlock,
				Destination:     common.String(infrastructurev1beta2.WorkerSubnetDefaultCIDR),
			},
		},
	}
}
