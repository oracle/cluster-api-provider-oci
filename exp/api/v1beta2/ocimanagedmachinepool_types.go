package v1beta2

import (
	infrastructurev1beta2 "github.com/oracle/cluster-api-provider-oci/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ManagedMachinePoolFinalizer is the finalizer for managed machine pool.
	ManagedMachinePoolFinalizer = "ocimanagedmachinepool.infrastructure.cluster.x-k8s.io"
)

// OCIManagedMachinePoolSpec defines the desired state of an OCI managed machine pool.
// An OCIManagedMachinePool translates to an OKE NodePool.
// The properties are generated from https://docs.oracle.com/en-us/iaas/api/#/en/containerengine/20180222/datatypes/CreateNodePoolDetails
type OCIManagedMachinePoolSpec struct {

	// ProviderID is the OCID of the associated NodePool in a provider format
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Version represents the version of the OKE node pool.
	Version *string `json:"version,omitempty"`

	// ID is the OCID of the associated NodePool
	// +optional
	ID *string `json:"id,omitempty"`

	// NodePoolNodeConfig defines the configuration of nodes in the node pool.
	// +optional
	NodePoolNodeConfig *NodePoolNodeConfig `json:"nodePoolNodeConfig,omitempty"`

	// NodeEvictionNodePoolSettings defines the eviction settings.
	// +optional
	NodeEvictionNodePoolSettings *NodeEvictionNodePoolSettings `json:"nodeEvictionNodePoolSettings,omitempty"`

	// NodeShape defines the name of the node shape of the nodes in the node pool.
	// +optional
	NodeShape string `json:"nodeShape,omitempty"`

	// NodeShapeConfig defines the configuration of the shape to launch nodes in the node pool.
	// +optional
	NodeShapeConfig *NodeShapeConfig `json:"nodeShapeConfig,omitempty"`

	// NodeSourceViaImage defines the image configuration of the nodes in the nodepool.
	// +optional
	NodeSourceViaImage *NodeSourceViaImage `json:"nodeSourceViaImage,omitempty"`

	// SshPublicKey defines the SSH public key on each node in the node pool on launch.
	// +optional
	SshPublicKey string `json:"sshPublicKey,omitempty"`

	// NodeMetadata defines a list of key/value pairs to add to each underlying OCI instance in the node pool on launch.
	// +optional
	NodeMetadata map[string]string `json:"nodeMetadata,omitempty"`

	// InitialNodeLabels defines a list of key/value pairs to add to nodes after they join the Kubernetes cluster.
	// +optional
	InitialNodeLabels []KeyValue `json:"initialNodeLabels,omitempty"`

	// NodePoolCyclingDetails defines the node pool recycling options.
	// +optional
	NodePoolCyclingDetails *NodePoolCyclingDetails `json:"nodePoolCyclingDetails,omitempty"`

	// ProviderIDList are the identification IDs of machine instances provided by the provider.
	// This field must match the provider IDs as seen on the node objects corresponding to a machine pool's machine instances.
	// +optional
	ProviderIDList []string `json:"providerIDList,omitempty"`
}

// NodePoolNodeConfig describes the configuration of nodes in the node pool.
type NodePoolNodeConfig struct {
	// IsPvEncryptionInTransitEnabled defines whether in transit encryption should be enabled on the nodes.
	// +optional
	IsPvEncryptionInTransitEnabled *bool `json:"isPvEncryptionInTransitEnabled,omitempty"`

	// KmsKeyId  defines whether in transit encryption should be enabled on the nodes.
	// +optional
	KmsKeyId *string `json:"kmsKeyId,omitempty"`

	// PlacementConfigs defines the placement configurations for the node pool.
	// +optional
	PlacementConfigs []PlacementConfig `json:"placementConfigs,omitempty"`

	// NsgNames defines the names of NSGs which will be associated with the nodes. the NSGs are defined
	// in OCIManagedCluster object.
	// +optional
	NsgNames []string `json:"nsgNames,omitempty"`

	// NodePoolPodNetworkOptionDetails defines the pod networking details of the node pool
	// +optional
	NodePoolPodNetworkOptionDetails *NodePoolPodNetworkOptionDetails `json:"nodePoolPodNetworkOptionDetails,omitempty"`
}

// NodePoolPodNetworkOptionDetails describes the CNI related configuration of pods in the node pool.
type NodePoolPodNetworkOptionDetails struct {

	// CniType describes the CNI plugin used by this node pool. Allowed values are OCI_VCN_IP_NATIVE and FLANNEL_OVERLAY.
	// +optional
	CniType infrastructurev1beta2.CNIOptionEnum `json:"cniType,omitempty"`

	// VcnIpNativePodNetworkOptions describes the network options specific to using the OCI VCN Native CNI
	// +optional
	VcnIpNativePodNetworkOptions VcnIpNativePodNetworkOptions `json:"vcnIpNativePodNetworkOptions,omitempty"`
}

// VcnIpNativePodNetworkOptions defines the Network options specific to using the OCI VCN Native CNI
type VcnIpNativePodNetworkOptions struct {

	// MemoryInGBs defines the max number of pods per node in the node pool. This value will be limited by the
	// number of VNICs attachable to the node pool shape
	// +optional
	MaxPodsPerNode *int `json:"maxPodsPerNode,omitempty"`

	// NSGNames defines the NSGs associated with the native pod network.
	// +optional
	NSGNames []string `json:"nsgNames,omitempty"`

	// SubnetNames defines the Subnets associated with the native pod network.
	// +optional
	SubnetNames []string `json:"subnetNames,omitempty"`
}

// PlacementConfig defines the placement configurations for the node pool.
type PlacementConfig struct {
	// AvailabilityDomain defines the availability domain in which to place nodes.
	// +optional
	AvailabilityDomain *string `json:"availabilityDomain,omitempty"`

	// CapacityReservationId defines the OCID of the compute capacity reservation in which to place the compute instance.
	// +optional
	CapacityReservationId *string `json:"capacityReservationId,omitempty"`

	// FaultDomains defines the list of fault domains in which to place nodes.
	// +optional
	FaultDomains []string `json:"faultDomains,omitempty"`

	// SubnetName defines the name of the subnet which need ot be associated with the Nodepool.
	// The subnets are defined in the OCiManagedCluster object.
	// +optional
	SubnetName *string `json:"subnetName,omitempty"`
}

// NodeEvictionNodePoolSettings defines the Node Eviction Details configuration.
type NodeEvictionNodePoolSettings struct {

	// EvictionGraceDuration defines the duration after which OKE will give up eviction of the pods on the node. PT0M will indicate you want to delete the node without cordon and drain. Default PT60M, Min PT0M, Max: PT60M. Format ISO 8601 e.g PT30M
	// +optional
	EvictionGraceDuration *string `json:"evictionGraceDuration,omitempty"`

	// IsForceDeleteAfterGraceDuration defines if the underlying compute instance should be deleted if you cannot evict all the pods in grace period
	// +optional
	IsForceDeleteAfterGraceDuration *bool `json:"isForceDeleteAfterGraceDuration,omitempty"`
}

// NodeShapeConfig defines the shape configuration of the nodes.
type NodeShapeConfig struct {

	// MemoryInGBs defines the total amount of memory available to each node, in gigabytes.
	// +optional
	MemoryInGBs *string `json:"memoryInGBs,omitempty"`

	// Ocpus defines the total number of OCPUs available to each node in the node pool.
	// +optional
	Ocpus *string `json:"ocpus,omitempty"`
}

// NodeSourceViaImage defines the Details of the image running on the node.
type NodeSourceViaImage struct {

	// BootVolumeSizeInGBs defines the size of the boot volume in GBs.
	// +optional
	BootVolumeSizeInGBs *int64 `json:"bootVolumeSizeInGBs,omitempty"`

	// ImageId defines the OCID of the image used to boot the node.
	// +optional
	ImageId *string `json:"imageId,omitempty"`
}

// KeyValue The properties that define a key value pair.
type KeyValue struct {

	// The key of the pair.
	Key *string `json:"key,omitempty"`

	// The value of the pair.
	Value *string `json:"value,omitempty"`
}

// NodePoolCyclingDetails defines the node pool recycling options
type NodePoolCyclingDetails struct {

	// IsNodeCyclingEnabled refers if nodes in the nodepool will be cycled to have new changes.
	// +optional
	IsNodeCyclingEnabled *bool `json:"isNodeCyclingEnabled,omitempty"`

	// MaximumSurge refers to the maximum additional new compute instances that would be temporarily created and
	// added to nodepool during the cycling nodepool process. OKE supports both integer and percentage input.
	// Defaults to 1, Ranges from 0 to Nodepool size or 0% to 100%
	// +optional
	MaximumSurge *string `json:"maximumSurge,omitempty"`

	// Maximum active nodes that would be terminated from nodepool during the cycling nodepool process.
	// OKE supports both integer and percentage input. Defaults to 0, Ranges from 0 to Nodepool size or 0% to 100%
	// +optional
	MaximumUnavailable *string `json:"maximumUnavailable,omitempty"`
}

// OCIManagedMachinePoolStatus defines the observed state of OCIManagedMachinePool
type OCIManagedMachinePoolStatus struct {
	// +optional
	Ready bool `json:"ready"`
	// NetworkSpec encapsulates all things related to OCI network.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// +optional
	NodepoolLifecycleState string `json:"nodepoolLifecycleState,omitempty"`

	// Replicas is the most recently observed number of replicas
	// +optional
	Replicas int32 `json:"replicas"`

	FailureReason *string `json:"failureReason,omitempty"`

	FailureMessages []string `json:"failureMessages,omitempty"`

	// InfrastructureMachineKind is the kind of the infrastructure resources behind MachinePool Machines.
	// +optional
	InfrastructureMachineKind string `json:"infrastructureMachineKind,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion

// OCIManagedMachinePool is the Schema for the ocimanagedmachinepool API.
type OCIManagedMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIManagedMachinePoolSpec   `json:"spec,omitempty"`
	Status OCIManagedMachinePoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:storageversion

// OCIManagedMachinePoolList contains a list of OCIManagedMachinePool.
type OCIManagedMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIManagedMachinePool `json:"items"`
}

// GetConditions returns the list of conditions for an OCIMachine API object.
func (m *OCIManagedMachinePool) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions will set the given conditions on an OCIMachine object.
func (m *OCIManagedMachinePool) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&OCIManagedMachinePool{}, &OCIManagedMachinePoolList{})
}
