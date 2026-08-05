package types

import (
	"time"

	hyperfleetv1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1"
)

// NodePool represents a nodepool resource
type NodePool struct {
	ID              string                          `json:"id"`
	ClusterID       string                          `json:"cluster_id"`
	Name            string                          `json:"name"`
	CreatedBy       string                          `json:"created_by"`
	Generation      int64                           `json:"generation"`
	ResourceVersion string                          `json:"resource_version"`
	Spec            hyperfleetv1alpha1.NodePoolSpec `json:"spec"`
	Status          *NodePoolStatusInfo             `json:"status,omitempty"`
	CreatedAt       time.Time                       `json:"created_at"`
	UpdatedAt       time.Time                       `json:"updated_at"`
}

// NodePoolCreateRequest represents a request to create a nodepool
type NodePoolCreateRequest struct {
	ClusterID string                           `json:"cluster_id"`
	Name      string                           `json:"name"`
	Spec      *hyperfleetv1alpha1.NodePoolSpec `json:"spec"`
}

// NodePoolUpdateRequest represents a request to update a nodepool
type NodePoolUpdateRequest struct {
	Spec *hyperfleetv1alpha1.NodePoolSpec `json:"spec"`
}

// NodePoolStatusInfo represents the status of a nodepool
type NodePoolStatusInfo struct {
	ObservedGeneration int64       `json:"observedGeneration"`
	Conditions         []Condition `json:"conditions,omitempty"`
	Phase              string      `json:"phase"`
	Message            string      `json:"message,omitempty"`
	Reason             string      `json:"reason,omitempty"`
	LastUpdateTime     time.Time   `json:"lastUpdateTime"`
}

// NodePoolControllerStatus represents controller-specific status for a nodepool
type NodePoolControllerStatus struct {
	NodePoolID         string         `json:"nodepool_id"`
	ControllerName     string         `json:"controller_name"`
	ObservedGeneration int64          `json:"observed_generation"`
	Conditions         []Condition    `json:"conditions,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	LastUpdated        time.Time      `json:"last_updated"`
}

// NodePoolStatusResponse represents the response for nodepool status endpoint
type NodePoolStatusResponse struct {
	NodePoolID         string                      `json:"nodepool_id"`
	Status             *NodePoolStatusInfo         `json:"status"`
	ControllerStatuses []*NodePoolControllerStatus `json:"controller_statuses,omitempty"`
}
