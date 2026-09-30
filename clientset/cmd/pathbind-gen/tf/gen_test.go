package tf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGeneratesBundledTerraformObject(t *testing.T) {
	dir := t.TempDir()
	draft := filepath.Join(dir, "draft.yaml")
	overrides := filepath.Join(dir, "overrides.yaml")
	output := filepath.Join(dir, "generated")
	if err := os.WriteFile(draft, []byte(`resources:
  cluster:
    sdkType: v1alpha1.Cluster
    fields:
      - path: metadata.uid
        goType: string
        operations: [create]
      - path: spec.hostedCluster.networking.networkType
        goType: string
        operations: [create]
      - path: spec.hostedCluster.networking.apiServer.advertiseAddress
        goType: string
        operations: [create]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overrides, []byte(`config:
  package: generated
  tfProviderPkg: example/provider
resources:
  cluster:
    aliases:
      - path: spec.hostedCluster.networking.networkType
        bundle: network
      - path: spec.hostedCluster.networking.apiServer.advertiseAddress
        bundle: network
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Run(draft, overrides, output); err != nil {
		t.Fatal(err)
	}
	native, err := os.ReadFile(filepath.Join(output, "cluster_state_native_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(native), "ClusterNetworkNative") {
		t.Fatalf("generated native state did not contain bundle:\n%s", native)
	}
	state, err := os.ReadFile(filepath.Join(output, "cluster_state_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), `Network types.Object `+"`tfsdk:\"network\"`") {
		t.Fatalf("generated Terraform state did not contain network object:\n%s", state)
	}
	resource, err := os.ReadFile(filepath.Join(output, "cluster_resource_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	resourceText := string(resource)
	for _, want := range []string{
		`"network": schema.SingleNestedAttribute`,
		`"network_type": schema.StringAttribute`,
		`objectToNative(tf.Network`,
		`nativeBundleAttributes(native.Network`,
	} {
		if !strings.Contains(resourceText, want) {
			t.Errorf("generated resource missing %q", want)
		}
	}
}
