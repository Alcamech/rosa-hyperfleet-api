/*
Copyright 2026.

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

package v1alpha1

import (
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NodePoolPhase represents the lifecycle phase of a NodePool.
// +kubebuilder:validation:Enum=WaitingForCluster;Provisioning;Ready;Deleting
type NodePoolPhase string

const (
	NodePoolPhaseWaitingForCluster NodePoolPhase = "WaitingForCluster"
	NodePoolPhaseProvisioning      NodePoolPhase = "Provisioning"
	NodePoolPhaseReady             NodePoolPhase = "Ready"
	NodePoolPhaseDeleting          NodePoolPhase = "Deleting"
)

// NodePoolSpec defines the desired state of a NodePool.
// The parent Cluster is identified by the shared metadata.Namespace (cluster UUID).
type NodePoolSpec struct {
	// DisplayName is a human-readable name for the node pool.
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxLength=256
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// AutoRepair controls whether unhealthy nodes are automatically replaced.
	// +hyperfleet:write-mode=mutable
	// +optional
	AutoRepair *bool `json:"autoRepair,omitempty"`

	// Labels are customer-defined labels applied to nodes in this pool.
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxProperties=100
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// AccountID is the AWS account that owns this node pool.
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	// +optional
	AccountID string `json:"accountId,omitempty"`

	// InternalPoolID is a platform-assigned unique identifier.
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	// +optional
	InternalPoolID string `json:"internalPoolId,omitempty"`

	// NodePool contains the upstream HyperShift fields, mirrored as
	// passthrough types with per-field visibility and write-mode markers.
	// +kubebuilder:validation:Required
	NodePool NodePoolSpecPassthrough `json:"nodePool"`
}

// NodePoolStatus defines the observed state of a NodePool.
type NodePoolStatus struct {
	// Replicas is the latest observed number of nodes in the pool.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Conditions represent the latest observations of the node pool's state.
	// Known condition types: Synced, Ready.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase summarizes the node pool's lifecycle state.
	// +optional
	Phase NodePoolPhase `json:"phase,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +genclient
// +bridge:watch=disabled
// +bridge:wait
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=hfnp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// NodePool is the Schema for the nodepools API.
// It represents a set of worker nodes for a Cluster.
// The parent Cluster shares the same metadata.Namespace (cluster UUID).
type NodePool struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec NodePoolSpec `json:"spec"`

	// +optional
	Status NodePoolStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// NodePoolList contains a list of NodePool.
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []NodePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &NodePool{}, &NodePoolList{})
		return nil
	})
}

// NodePoolPlatform mirrors NodePoolPlatform from upstream HyperShift, exposing only
// the AWS platform. Other platforms are intentionally omitted.
// +hyperfleet:upstream-reduced-object=hypershiftv1beta1.NodePoolPlatform
// +k8s:openapi-gen=true
type NodePoolPlatform struct {
	// type specifies the platform type for this NodePool.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=immutable
	// +required
	Type hypershiftv1beta1.PlatformType `json:"type"`

	// aws specifies the platform-specific AWS configuration for this NodePool.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +optional
	AWS *AWSNodePoolPlatform `json:"aws,omitempty"`
}

// AWSNodePoolPlatform mirrors upstream HyperShift's type, reduced further so RootVolume's sub-fields
// can carry independent write-modes (see Volume); enforcement happens via FieldValidator at request time.
// +hyperfleet:upstream-reduced-object=hypershiftv1beta1.AWSNodePoolPlatform
// +k8s:openapi-gen=true
type AWSNodePoolPlatform struct {
	// instanceType is an ec2 instance type for node instances (e.g. m5.large).
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxLength=255
	// +required
	InstanceType string `json:"instanceType"`

	// instanceProfile is the AWS EC2 instance profile, which is a container for an IAM role that the EC2 instance uses.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxLength=255
	// +optional
	InstanceProfile string `json:"instanceProfile,omitempty"`

	// subnet is the subnet to use for node instances.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:XValidation:rule="has(self.id) && self.id.startsWith('subnet-') ? !has(self.filters) : size(self.filters) > 0", message="subnet is invalid, a valid subnet id or filters must be set, but not both"
	// +required
	Subnet hypershiftv1beta1.AWSResourceReference `json:"subnet"`

	// ami is the image id to use for node instances. If unspecified, the default
	// is chosen based on the NodePool release payload image.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxLength=255
	// +optional
	AMI string `json:"ami,omitempty"`

	// imageType specifies the type of image to use for node instances.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:Enum=Windows;Linux
	// +optional
	ImageType hypershiftv1beta1.ImageType `json:"imageType,omitempty"`

	// securityGroups is an optional set of security groups to associate with node
	// instances.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxItems=50
	// +optional
	SecurityGroups []hypershiftv1beta1.AWSResourceReference `json:"securityGroups,omitempty"`

	// rootVolume specifies configuration for the root volume of node instances.
	// Reduced one level further — see the Volume type.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +optional
	RootVolume *Volume `json:"rootVolume,omitempty"`

	// resourceTags is an optional list of additional tags to apply to AWS node
	// instances.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxItems=25
	// +optional
	ResourceTags []hypershiftv1beta1.AWSResourceTag `json:"resourceTags,omitempty"`

	// placement specifies the placement options for the EC2 instances.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=mutable
	// +optional
	Placement *hypershiftv1beta1.PlacementOptions `json:"placement,omitempty"`
}

// Volume mirrors upstream HyperShift's Volume type. Write-modes match v1: only size is customer-settable
// (immutable after creation); type/iops/encrypted/encryptionKey are service-set, enforced by FieldValidator.
// +hyperfleet:upstream-reduced-object=hypershiftv1beta1.Volume
// +k8s:openapi-gen=true
type Volume struct {
	// size is the size of the volume in gibibytes (GiB).
	// Settable on create only — matches v1, which rejects changing this on update.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=immutable
	// +kubebuilder:validation:Minimum=8
	// +required
	Size int64 `json:"size"`

	// type is computed by the platform, matching v1 (always gp3). A customer-supplied non-default value is
	// rejected at request validation rather than hidden from the schema.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=service-set
	// +kubebuilder:validation:MaxLength=255
	Type string `json:"type"`

	// iops is computed by the platform, matching v1 (always 0 for gp2/gp3). A customer-supplied non-default
	// value is rejected at request validation rather than hidden from the schema.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=service-set
	IOPS int64 `json:"iops,omitempty"`

	// encrypted is computed by the platform, matching v1 (always true). A customer-supplied non-default
	// value is rejected at request validation rather than hidden from the schema.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=service-set
	Encrypted *bool `json:"encrypted,omitempty"`

	// encryptionKey is computed by the platform, matching v1 (never set). A customer-supplied non-default
	// value is rejected at request validation rather than hidden from the schema.
	// +k8s:openapi-gen=true
	// +hyperfleet:write-mode=service-set
	// +kubebuilder:validation:MaxLength=2048
	EncryptionKey string `json:"encryptionKey,omitempty"`
}
