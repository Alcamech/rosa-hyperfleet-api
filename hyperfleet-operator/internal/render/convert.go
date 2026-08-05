package render

import (
	"encoding/json"
	"fmt"

	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	hyperfleetv1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1"
)

func toHostedClusterSpec(p *hyperfleetv1alpha1.HostedClusterSpecPassthrough) (*hypershiftv1beta1.HostedClusterSpec, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshaling HostedClusterSpecPassthrough: %w", err)
	}
	var spec hypershiftv1beta1.HostedClusterSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshalling to HostedClusterSpec: %w", err)
	}
	return &spec, nil
}

func toNodePoolSpec(p *hyperfleetv1alpha1.NodePoolSpecPassthrough) (*hypershiftv1beta1.NodePoolSpec, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshaling NodePoolSpecPassthrough: %w", err)
	}
	var spec hypershiftv1beta1.NodePoolSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshalling to NodePoolSpec: %w", err)
	}
	return &spec, nil
}
