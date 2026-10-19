package featuregate

// SeaplaneFeatureGates is the registry of all feature gates
// Each gate controls access to specific fields or capabilities
var SeaplaneFeatureGates = map[string]FeatureGateInfo{
	// Example gates - these would be populated based on actual product requirements

	"SeaplaneEtcdConfig": {
		Stage:       GA,
		Description: "Allows customers to configure etcd settings",
	},

	"SeaplaneAutoScaling": {
		Stage:       TechPreview,
		Description: "Enables cluster autoscaling configuration",
	},

	"SeaplaneSecretEncryption": {
		Stage:       TechPreview,
		Description: "Allows customers to configure secret encryption",
	},

	"SeaplaneCustomDNS": {
		Stage:       DevPreview,
		Description: "Enables custom DNS configuration for development/testing",
	},

	"SeaplaneKubeletAdvanced": {
		Stage:       TechPreview,
		Description: "Enables advanced kubelet configuration (serializeImagePulls, registryPullQPS, etc.)",
	},

	"SeaplaneMachineConfig": {
		Stage:       TechPreview,
		Description: "Allows customers to request approved kernel parameters via allowlist",
	},
}

// IsGateEnabled returns true if the given gate is enabled for the feature set
func IsGateEnabled(gate string, featureSet FeatureSet) bool {
	info, exists := SeaplaneFeatureGates[gate]
	if !exists {
		// Unknown gates are disabled by default
		return false
	}

	return featureSet.Includes(info.Stage)
}

// GatesForFeatureSet returns all gates enabled for the given feature set
func GatesForFeatureSet(featureSet FeatureSet) []string {
	var gates []string
	maxStage := featureSet.MaxStage()

	for gate, info := range SeaplaneFeatureGates {
		if info.Stage <= maxStage {
			gates = append(gates, gate)
		}
	}

	return gates
}
