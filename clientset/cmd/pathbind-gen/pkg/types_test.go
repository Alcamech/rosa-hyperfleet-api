package pkg

import "testing"

func TestBuildMergedAliasesPreservesBundle(t *testing.T) {
	draft := map[string]DraftField{
		"spec.hostedCluster.networking.machineNetwork": {Path: "spec.hostedCluster.networking.machineNetwork", GoType: "string", Operations: []string{"create"}},
	}
	aliases, err := BuildMergedAliases(draft, []OverrideAlias{{
		Path:   "spec.hostedCluster.networking.machineNetwork",
		Bundle: "network",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 1 || aliases[0].Bundle != "network" {
		t.Fatalf("bundle was not preserved: %#v", aliases)
	}
}
