package v1alpha1

import (
	"testing"

	crd "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1"
)

func TestProjectNodePoolObservedReplicas(t *testing.T) {
	replicas := int32(0)
	projected := ProjectNodePool(&crd.NodePool{
		Status: crd.NodePoolStatus{Replicas: &replicas},
	})
	if projected.Status.Replicas == nil || *projected.Status.Replicas != 0 {
		t.Fatalf("projected replicas = %v, want pointer to zero", projected.Status.Replicas)
	}

	unknown := ProjectNodePool(&crd.NodePool{})
	if unknown.Status.Replicas != nil {
		t.Fatalf("projected replicas = %v, want nil", unknown.Status.Replicas)
	}
}
