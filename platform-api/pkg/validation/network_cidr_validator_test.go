package validation

import (
	"testing"

	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/api/util/ipnet"
)

func TestValidateNetworkCIDRs(t *testing.T) {
	tests := []struct {
		name       string
		networking *hypershiftv1beta1.ClusterNetworking
		wantErr    bool
		errField   string
	}{
		{
			name: "valid network configuration",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/16")},
				},
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/14"), HostPrefix: 23},
				},
				ServiceNetwork: []hypershiftv1beta1.ServiceNetworkEntry{
					{CIDR: mustParseCIDR("172.30.0.0/16")},
				},
			},
			wantErr: false,
		},
		{
			name: "machine and service CIDR overlap",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/16")},
				},
				ServiceNetwork: []hypershiftv1beta1.ServiceNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/24")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking",
		},
		{
			name: "machine and pod CIDR overlap",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/16")},
				},
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/14"), HostPrefix: 23},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking",
		},
		{
			name: "service and pod CIDR overlap",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/14"), HostPrefix: 23},
				},
				ServiceNetwork: []hypershiftv1beta1.ServiceNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/24")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking",
		},
		{
			name: "invalid host prefix - too small",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/14"), HostPrefix: 22},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.clusterNetwork.hostPrefix",
		},
		{
			name: "invalid host prefix - too large",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/14"), HostPrefix: 27},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.clusterNetwork.hostPrefix",
		},
		{
			name: "insufficient pod nodes",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/20"), HostPrefix: 23},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.clusterNetwork",
		},
		{
			name: "machine CIDR contains HyperShift API address",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("172.20.0.0/16")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.machineNetwork",
		},
		{
			name: "service CIDR contains HyperShift API address",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ServiceNetwork: []hypershiftv1beta1.ServiceNetworkEntry{
					{CIDR: mustParseCIDR("172.20.0.0/16")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.serviceNetwork",
		},
		{
			name: "CIDR address inconsistent with prefix",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.5/16")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.machineNetwork",
		},
		{
			name: "invalid CIDR format",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseInvalidCIDR("invalid")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.machineNetwork",
		},
		{
			name: "service CIDR too small",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ServiceNetwork: []hypershiftv1beta1.ServiceNetworkEntry{
					{CIDR: mustParseCIDR("172.30.0.0/25")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.serviceNetwork",
		},
		{
			name: "pod CIDR too small",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/22"), HostPrefix: 23},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.clusterNetwork",
		},
		{
			name:       "nil networking",
			networking: nil,
			wantErr:    false,
		},
		{
			name: "multiple machine networks with overlap",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/16")},
					{CIDR: mustParseCIDR("10.0.0.0/24")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.machineNetwork",
		},
		{
			name: "multiple service networks with overlap",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ServiceNetwork: []hypershiftv1beta1.ServiceNetworkEntry{
					{CIDR: mustParseCIDR("172.30.0.0/16")},
					{CIDR: mustParseCIDR("172.30.0.0/24")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.serviceNetwork",
		},
		{
			name: "multiple pod networks with overlap",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/14"), HostPrefix: 23},
					{CIDR: mustParseCIDR("10.128.0.0/16"), HostPrefix: 23},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.clusterNetwork",
		},
		{
			name: "valid multiple networks without overlap",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/16")},
					{CIDR: mustParseCIDR("192.168.0.0/16")},
				},
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/14"), HostPrefix: 23},
					{CIDR: mustParseCIDR("10.132.0.0/14"), HostPrefix: 23},
				},
				ServiceNetwork: []hypershiftv1beta1.ServiceNetworkEntry{
					{CIDR: mustParseCIDR("172.30.0.0/16")},
					{CIDR: mustParseCIDR("172.31.0.0/16")},
				},
			},
			wantErr: false,
		},
		{
			name: "second pod network has invalid host prefix",
			networking: &hypershiftv1beta1.ClusterNetworking{
				ClusterNetwork: []hypershiftv1beta1.ClusterNetworkEntry{
					{CIDR: mustParseCIDR("10.128.0.0/14"), HostPrefix: 23},
					{CIDR: mustParseCIDR("10.132.0.0/14"), HostPrefix: 22},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.clusterNetwork.hostPrefix",
		},
		{
			name: "HyperShift API collision in second machine network",
			networking: &hypershiftv1beta1.ClusterNetworking{
				MachineNetwork: []hypershiftv1beta1.MachineNetworkEntry{
					{CIDR: mustParseCIDR("10.0.0.0/16")},
					{CIDR: mustParseCIDR("172.20.0.0/16")},
				},
			},
			wantErr:  true,
			errField: "spec.hostedCluster.networking.machineNetwork",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewNetworkCIDRValidator()
			errs := validator.ValidateNetworkCIDRs(tt.networking)

			if tt.wantErr {
				if errs == nil {
					t.Errorf("ValidateNetworkCIDRs() expected error but got none")
					return
				}
				if tt.errField != "" {
					found := false
					for _, err := range errs {
						if err.Field == tt.errField {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("ValidateNetworkCIDRs() expected error for field %s, but got errors for: %v", tt.errField, errs)
					}
				}
			} else {
				if errs != nil {
					t.Errorf("ValidateNetworkCIDRs() unexpected error: %v", errs)
				}
			}
		})
	}
}

func mustParseCIDR(cidr string) ipnet.IPNet {
	parsed, err := ipnet.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return *parsed
}

func mustParseInvalidCIDR(cidr string) ipnet.IPNet {
	// Create an invalid IPNet for testing
	return ipnet.IPNet{}
}
