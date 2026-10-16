package validation

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

type CIDRType string

const (
	MachineCIDR          CIDRType = "Machine CIDR"
	ServiceCIDR          CIDRType = "Service CIDR"
	PodCIDR              CIDRType = "Pod CIDR"
	ServiceCIDRSubnetMax          = 24
	PodCIDRSubnetMax              = 21
	PodNodesMin                   = 32
	HostPrefixMin                 = 23
	HostPrefixMax                 = 26

	// DefaultHypershiftAdvertiseAddress is the default IP on which Hypershift advertises the API server
	DefaultHypershiftAdvertiseAddress = "172.20.0.1"
)

type NetworkCIDRValidator struct{}

func NewNetworkCIDRValidator() *NetworkCIDRValidator {
	return &NetworkCIDRValidator{}
}

// ValidateNetworkCIDRs validates the network configuration for a cluster
func (v *NetworkCIDRValidator) ValidateNetworkCIDRs(networking *hypershiftv1beta1.ClusterNetworking) ValidationErrors {
	if networking == nil {
		return nil
	}

	var errs ValidationErrors
	machineNets, serviceNets, podNets := v.parseNetworks(networking, &errs)

	// Check for network overlaps within and across all network lists
	errs = append(errs, v.validateNetworkOverlaps(machineNets, serviceNets, podNets)...)

	// Check for HyperShift API address collision against all networks
	errs = append(errs, v.validateHypershiftAPICollision(machineNets, serviceNets, podNets)...)

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (v *NetworkCIDRValidator) parseNetworks(networking *hypershiftv1beta1.ClusterNetworking, errs *ValidationErrors) ([]*net.IPNet, []*net.IPNet, []*net.IPNet) {
	var machineNets, serviceNets, podNets []*net.IPNet

	// Validate and parse all Machine CIDRs
	for _, entry := range networking.MachineNetwork {
		if entry.CIDR.String() != "" {
			parsed, err := v.parseNetworkCIDR(entry.CIDR.String(), MachineCIDR)
			if err != nil {
				*errs = append(*errs, err)
			} else {
				machineNets = append(machineNets, parsed)
			}
		}
	}

	// Validate and parse all Service CIDRs
	for _, entry := range networking.ServiceNetwork {
		if entry.CIDR.String() != "" {
			parsed, err := v.parseNetworkCIDR(entry.CIDR.String(), ServiceCIDR)
			if err != nil {
				*errs = append(*errs, err)
			} else {
				serviceNets = append(serviceNets, parsed)
			}
		}
	}

	// Validate and parse all Pod CIDRs (ClusterNetwork)
	for _, entry := range networking.ClusterNetwork {
		if entry.CIDR.String() != "" {
			parsed, err := v.parseNetworkCIDR(entry.CIDR.String(), PodCIDR)
			if err != nil {
				*errs = append(*errs, err)
			} else {
				podNets = append(podNets, parsed)
				// Validate host prefix for this successfully parsed network
				*errs = append(*errs, v.validateHostPrefix(entry.HostPrefix, parsed)...)
			}
		}
	}

	return machineNets, serviceNets, podNets
}

func (v *NetworkCIDRValidator) validateHostPrefix(hostPrefix int32, podNet *net.IPNet) ValidationErrors {
	var errs ValidationErrors

	if hostPrefix == 0 {
		return errs
	}

	if hostPrefix < HostPrefixMin || hostPrefix > HostPrefixMax {
		errs = append(errs, &ValidationError{
			Field:  "spec.hostedCluster.networking.clusterNetwork.hostPrefix",
			Reason: fmt.Sprintf("Invalid Network Host Prefix '%d': Subnet length should be between '%v' and '%v'", hostPrefix, HostPrefixMin, HostPrefixMax),
		})
	}

	// Validate pod nodes calculation
	if podNet != nil {
		podSubnet, _ := podNet.Mask.Size()
		podNodes := int64(math.Pow(2, float64(hostPrefix-int32(podSubnet))))
		if podNodes < PodNodesMin {
			errs = append(errs, &ValidationError{
				Field:  "spec.hostedCluster.networking.clusterNetwork",
				Reason: fmt.Sprintf("The pod subnet length '%d' with host prefix '%d' does not allow for enough nodes (minimum %d required, got %d)", podSubnet, hostPrefix, PodNodesMin, podNodes),
			})
		}
	}

	return errs
}

func (v *NetworkCIDRValidator) validateNetworkOverlaps(machineNets, serviceNets, podNets []*net.IPNet) ValidationErrors {
	var errs ValidationErrors

	// Check machine vs service overlaps
	for _, machineNet := range machineNets {
		for _, serviceNet := range serviceNets {
			if v.intersect(machineNet, serviceNet) {
				errs = append(errs, &ValidationError{
					Field:  "spec.hostedCluster.networking",
					Reason: fmt.Sprintf("Machine CIDR '%s' and Service CIDR '%s' overlap", machineNet.String(), serviceNet.String()),
				})
			}
		}
	}

	// Check machine vs pod overlaps
	for _, machineNet := range machineNets {
		for _, podNet := range podNets {
			if v.intersect(machineNet, podNet) {
				errs = append(errs, &ValidationError{
					Field:  "spec.hostedCluster.networking",
					Reason: fmt.Sprintf("Machine CIDR '%s' and Pod CIDR '%s' overlap", machineNet.String(), podNet.String()),
				})
			}
		}
	}

	// Check service vs pod overlaps
	for _, serviceNet := range serviceNets {
		for _, podNet := range podNets {
			if v.intersect(serviceNet, podNet) {
				errs = append(errs, &ValidationError{
					Field:  "spec.hostedCluster.networking",
					Reason: fmt.Sprintf("Service CIDR '%s' and Pod CIDR '%s' overlap", serviceNet.String(), podNet.String()),
				})
			}
		}
	}

	// Check overlaps within machine networks
	for i, net1 := range machineNets {
		for j, net2 := range machineNets {
			if i < j && v.intersect(net1, net2) {
				errs = append(errs, &ValidationError{
					Field:  "spec.hostedCluster.networking.machineNetwork",
					Reason: fmt.Sprintf("Machine CIDRs '%s' and '%s' overlap", net1.String(), net2.String()),
				})
			}
		}
	}

	// Check overlaps within service networks
	for i, net1 := range serviceNets {
		for j, net2 := range serviceNets {
			if i < j && v.intersect(net1, net2) {
				errs = append(errs, &ValidationError{
					Field:  "spec.hostedCluster.networking.serviceNetwork",
					Reason: fmt.Sprintf("Service CIDRs '%s' and '%s' overlap", net1.String(), net2.String()),
				})
			}
		}
	}

	// Check overlaps within pod networks
	for i, net1 := range podNets {
		for j, net2 := range podNets {
			if i < j && v.intersect(net1, net2) {
				errs = append(errs, &ValidationError{
					Field:  "spec.hostedCluster.networking.clusterNetwork",
					Reason: fmt.Sprintf("Pod CIDRs '%s' and '%s' overlap", net1.String(), net2.String()),
				})
			}
		}
	}

	return errs
}

func (v *NetworkCIDRValidator) validateHypershiftAPICollision(machineNets, serviceNets, podNets []*net.IPNet) ValidationErrors {
	var errs ValidationErrors

	hypershiftIP := net.ParseIP(DefaultHypershiftAdvertiseAddress)
	if hypershiftIP == nil {
		return errs
	}

	for _, network := range machineNets {
		if network != nil && network.Contains(hypershiftIP) {
			errs = append(errs, &ValidationError{
				Field:  "spec.hostedCluster.networking.machineNetwork",
				Reason: fmt.Sprintf("Machine CIDR should not contain the IP '%s' as it is reserved for internal usage", hypershiftIP),
			})
		}
	}

	for _, network := range serviceNets {
		if network != nil && network.Contains(hypershiftIP) {
			errs = append(errs, &ValidationError{
				Field:  "spec.hostedCluster.networking.serviceNetwork",
				Reason: fmt.Sprintf("Service CIDR should not contain the IP '%s' as it is reserved for internal usage", hypershiftIP),
			})
		}
	}

	for _, network := range podNets {
		if network != nil && network.Contains(hypershiftIP) {
			errs = append(errs, &ValidationError{
				Field:  "spec.hostedCluster.networking.clusterNetwork",
				Reason: fmt.Sprintf("Pod CIDR should not contain the IP '%s' as it is reserved for internal usage", hypershiftIP),
			})
		}
	}

	return errs
}

func (v *NetworkCIDRValidator) parseNetworkCIDR(cidr string, cidrType CIDRType) (*net.IPNet, *ValidationError) {
	// Parse the CIDR
	address, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, &ValidationError{
			Field:  v.cidrTypeToFieldPath(cidrType),
			Reason: fmt.Sprintf("Validating %s '%s' failed: %s", cidrType, cidr, err.Error()),
		}
	}

	// Validate CIDR maximum values
	if err := v.validateCIDRValue(cidr, cidrType); err != nil {
		return nil, &ValidationError{
			Field:  v.cidrTypeToFieldPath(cidrType),
			Reason: fmt.Sprintf("Validating %s '%s' failed: %s", cidrType, cidr, err.Error()),
		}
	}

	// Check that the address is consistent with the prefix
	original := address.String()
	masked := address.Mask(network.Mask).String()
	if original != masked {
		prefix, _ := network.Mask.Size()
		return nil, &ValidationError{
			Field:  v.cidrTypeToFieldPath(cidrType),
			Reason: fmt.Sprintf("Validating %s '%s' failed: network address '%s' isn't consistent with network prefix %d", cidrType, cidr, address, prefix),
		}
	}

	return network, nil
}

func (v *NetworkCIDRValidator) validateCIDRValue(cidr string, cidrType CIDRType) error {
	arr := strings.Split(cidr, "/")
	if len(arr) != 2 {
		return fmt.Errorf("invalid CIDR format")
	}

	var maxValue int
	switch cidrType {
	case MachineCIDR:
		// Machine CIDR is validated separately for each cloud provider
		return nil
	case ServiceCIDR:
		maxValue = ServiceCIDRSubnetMax
	case PodCIDR:
		maxValue = PodCIDRSubnetMax
	default:
		return nil
	}

	mCidr, err := strconv.Atoi(arr[1])
	if err != nil {
		return err
	}

	if mCidr > maxValue {
		return fmt.Errorf("%s value range is too small for correct provisioning. Netmask maximum allowed value is %d", cidrType, maxValue)
	}

	return nil
}

func (v *NetworkCIDRValidator) intersect(n1, n2 *net.IPNet) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

func (v *NetworkCIDRValidator) cidrTypeToFieldPath(cidrType CIDRType) string {
	switch cidrType {
	case MachineCIDR:
		return "spec.hostedCluster.networking.machineNetwork"
	case ServiceCIDR:
		return "spec.hostedCluster.networking.serviceNetwork"
	case PodCIDR:
		return "spec.hostedCluster.networking.clusterNetwork"
	default:
		return "spec.hostedCluster.networking"
	}
}
