package v1alpha1

import (
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	hostedClusterNamePrefix          = "hc-"
	managementClusterNamespacePrefix = "cluster-"
	clusterUIDLength                 = 36 // FleetDB UUID string length
)

// HostedClusterNameFromDNSReservation returns the internal HostedCluster name
// associated with a DNSReservation. Taking the resource rather than a UID
// string ensures the name is derived from a DNSReservation's own identity.
func HostedClusterNameFromDNSReservation(reservation *DNSReservation) string {
	if reservation == nil || reservation.UID == "" {
		return ""
	}
	nameLimit := MaxClusterNameLengthWithoutDNSReservation()
	if nameLimit <= len(hostedClusterNamePrefix) {
		return ""
	}

	var normalized strings.Builder
	for _, char := range strings.ToLower(string(reservation.UID)) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			normalized.WriteRune(char)
		}
	}

	suffix := normalized.String()
	maxSuffixLen := nameLimit - len(hostedClusterNamePrefix)
	if len(suffix) > maxSuffixLen {
		suffix = suffix[:maxSuffixLen]
	}
	if suffix == "" {
		return ""
	}

	return hostedClusterNamePrefix + suffix
}

// MaxClusterNameLengthWithoutDNSReservation returns the DNS-label name budget
// remaining after the management namespace prefix, UUID, and separator. The
// same budget caps reservation-derived internal HostedCluster names.
func MaxClusterNameLengthWithoutDNSReservation() int {
	return k8svalidation.DNS1123LabelMaxLength - len(managementClusterNamespacePrefix) - clusterUIDLength - 1
}
