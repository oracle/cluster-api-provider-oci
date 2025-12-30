package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

const (
	// VirtualMachinePoolFinalizer is the finalizer for virtual machine pool.
	VirtualMachinePoolFinalizer = "ocivirtualmachinepool.infrastructure.cluster.x-k8s.io"
)

// OCIVirtualMachinePoolSpec defines the desired state of an OCI virtual machine pool.
// An OCIVirtualMachinePool translates to an OKE Virtual node poo;.
// The properties are generated from https://docs.oracle.com/en-us/iaas/api/#/en/containerengine/20180222/datatypes/CreateVirtualNodePoolDetails
type OCIVirtualMachinePoolSpec struct {

	// ProviderID is the OCID of the associated NodePool in a provider format
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// ID is the OCID of the associated NodePool
	// +optional
	ID *string `json:"id,omitempty"`

	// PlacementConfigs defines the placement configurations for the node pool.
	// +optional
	PlacementConfigs []VirtualNodepoolPlacementConfig `json:"placementConfigs,omitempty"`

	// NsgNames defines the names of NSGs which will be associated with the nodes. the NSGs are defined
	// in OCIManagedCluster object.
	// +optional
	NsgNames []string `json:"nsgNames,omitempty"`

	// PodConfiguration defines pod configuration
	// +optional
	PodConfiguration PodConfig `json:"podConfiguration,omitempty"`

	// Taints describes the taints will be applied to the Virtual Nodes of this Virtual Node Pool for Kubernetes scheduling.
	// +optional
	Taints []Taint `json:"taints,omitempty"`

	// InitialVirtualNodeLabels defines a list of key/value pairs to add to nodes after they join the Kubernetes cluster.
	// +optional
	InitialVirtualNodeLabels []KeyValue `json:"initialVirtualNodeLabels,omitempty"`

	// ProviderIDList are the identification IDs of machine instances provided by the provider.
	// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`
}

// VirtualNodepoolPlacementConfig defines the placement configurations for the virtual node pool.
type VirtualNodepoolPlacementConfig struct {
	// AvailabilityDomain defines the availability domain in which to place nodes.
	// +optional

	AvailabilityDomain *string `json:"availabilityDomain,omitempty"`
	// FaultDomains defines the list of fault domains in which to place nodes.
	// +optional
	FaultDomains []string `json:"faultDomains,omitempty"`

	// SubnetName defines the name of the subnet which need to be associated with the Virtual Node Pool.
	// The subnets are defined in the OCiManagedCluster object.
	// +optional
	SubnetName *string `json:"subnetName,omitempty"`
}

// PodConfig  describes the pod configuration of the virtual node pool.
type PodConfig struct {
	// NsgNames defines the names of NSGs which will be associated with the pods.
	// +optional
	NsgNames []string `json:"nsgNames,omitempty"`

	// Shape described the shape of the pods.
	// +optional
	Shape *string `json:"shape,omitempty"`

	// SubnetName described the regional subnet where pods' VNIC will be placed.
	// +optional
	SubnetName *string `json:"subnetName,omitempty"`
}

// Taint describes a taint.
type Taint struct {
	// The key of the pair.
	Key *string `json:"key,omitempty"`

	// The value of the pair.
	Value *string `json:"value,omitempty"`

	// The effect of the pair.
	Effect *string `json:"effect,omitempty"`
}

// OCIVirtualMachinePoolStatus defines the observed state of OCIVirtualMachinePool
type OCIVirtualMachinePoolStatus struct {
	// +optional
	Ready bool `json:"ready"`
	// NetworkSpec encapsulates all things related to OCI network.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// Replicas is the most recently observed number of replicas
	// +optional
	Replicas int32 `json:"replicas"`

	// +optional
	NodepoolLifecycleState string `json:"nodepoolLifecycleState,omitempty"`

	// FailureReason will contains the CAPI MachinePoolStatusFailure if the virtual machine pool has hit an error condition.
	// +optional
	FailureReason *errors.MachinePoolStatusFailure `json:"failureReason,omitempty"`

	// FailureMessages contains the verbose erorr messages related to the virtual machine pool failures.
	// +optional
	FailureMessages []string `json:"failureMessages,omitempty"`

	// InfrastructureMachineKind is the kind of the infrastructure resources behind MachinePool Machines.
	// +optional
	InfrastructureMachineKind string `json:"infrastructureMachineKind,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// OCIVirtualMachinePool is the Schema for the ocivirtualmachinepool API.
type OCIVirtualMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIVirtualMachinePoolSpec   `json:"spec,omitempty"`
	Status OCIVirtualMachinePoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OCIVirtualMachinePoolList contains a list of OCIVirtualMachinePool.
type OCIVirtualMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIVirtualMachinePool `json:"items"`
}

// GetConditions returns the list of conditions for an OCIMachine API object.
func (m *OCIVirtualMachinePool) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions will set the given conditions on an OCIMachine object.
func (m *OCIVirtualMachinePool) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIVirtualMachinePool{}, &OCIVirtualMachinePoolList{})
}
