package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	v1 "github.com/openshift-online/rosa-seaplane-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// List shows every item; Describe decides full details or ID, name, and state.
func TestResourceCollectionsListDetails(t *testing.T) {
	for _, tc := range []struct {
		path, list, describe string
		index                int
	}{
		{"clusters", "ListClusters", "DescribeCluster", 0}, {"nodepools", "ListNodePools", "DescribeNodePool", 1}, {"oidc_configs", "ListOIDCConfigs", "DescribeOIDCConfig", 2}, {"dns_reservations", "ListDNSReservations", "DescribeDNSReservation", 3},
	} {
		for _, mode := range []string{"filtered", "list-only", "describe-only", "late-error"} {
			t.Run(tc.path+"/"+mode, func(t *testing.T) {
				objects := resourceFixtures()
				second := objects[tc.index].DeepCopyObject().(client.Object)
				second.SetName("second")
				second.SetUID(types.UID(string(objects[tc.index].GetUID()) + "-second"))
				labels := second.GetLabels()
				labels["team"], labels["fault"] = "red", "yes"
				second.SetLabels(labels)
				objects = append(objects, second)
				list := actionPermit(tc.list)
				describe := `permit(principal, action == Seaplane::Action::"` + tc.describe + `", resource) when { resource.hasTag("team") && resource.getTag("team") == "blue" };`
				policies := map[string][]string{
					"filtered":      {list, describe},
					"list-only":     {list},
					"describe-only": {describe},
					"late-error":    {list, describe, `forbid(principal, action == Seaplane::Action::"` + tc.describe + `", resource) when { resource.hasTag("fault") && 9223372036854775807 + 1 == 0 };`},
				}[mode]
				srv, _ := resourceServer(t, policies, interceptor.Funcs{}, objects...)
				w := resourceRequest(srv, "GET", tc.path, "")
				want := map[string]int{"filtered": 200, "list-only": 200, "describe-only": 403, "late-error": 500}[mode]
				if w.Code != want || (want != 200 && strings.Contains(w.Body.String(), `"items"`)) {
					t.Fatalf("want %d without items, got %d: %s", want, w.Code, w.Body.String())
				}
				if want != 200 {
					return
				}
				var body struct {
					Total int
					Items []json.RawMessage
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Total != 2 || len(body.Items) != 2 {
					t.Fatalf("want every item: %s (%v)", w.Body.String(), err)
				}
				detail := `"team"`
				for _, item := range body.Items {
					isFull := strings.Contains(string(item), detail)
					wantFull := mode == "filtered" && !strings.Contains(string(item), `"second"`)
					if isFull != wantFull {
						t.Fatalf("item details = %v, want %v: %s", isFull, wantFull, item)
					}
				}
			})
		}
	}
}

func TestNodePoolNamePinsParent(t *testing.T) {
	const secondID = "550e8400-e29b-41d4-a716-446655440001"
	objects := resourceFixtures()
	parent := objects[0].DeepCopyObject().(*v1.Cluster)
	parent.Name, parent.UID = "second-parent", types.UID(secondID)
	second := objects[1].DeepCopyObject().(*v1.NodePool)
	second.Name, second.UID = "second-parent.workers", "second-pool-uid"
	second.Spec.NodePool.ClusterName = parent.Name
	second.Labels["seaplane.io/cluster-uid"] = secondID
	second.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(parent, v1.GroupVersion.WithKind("Cluster"))}
	objects = append(objects, second, parent)
	hooks := interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		if err := c.List(ctx, list, opts...); err != nil {
			return err
		}
		if pools, ok := list.(*v1.NodePoolList); ok {
			sort.Slice(pools.Items, func(i, j int) bool { return pools.Items[i].Name < pools.Items[j].Name })
		}
		return nil
	}}
	policy := `permit(principal,action == Seaplane::Action::"DescribeNodePool",resource in Seaplane::Cluster::"123456789012/us-east-1/` + secondID + `");`
	srv, _ := resourceServer(t, []string{policy}, hooks, objects...)
	for _, tc := range []struct {
		path string
		want int
	}{
		{"parent.workers", 403},
		{"parent.workers?clusterId=" + resourceTestClusterID, 403},
		{"parent.workers?clusterId=" + secondID, 403},
		{"second-parent.workers", 200},
	} {
		w := resourceRequest(srv, "GET", "nodepools/"+tc.path, "")
		if w.Code != tc.want {
			t.Fatalf("path %s want %d got %d: %s", tc.path, tc.want, w.Code, w.Body.String())
		}
	}
	// List has independent collection permission; Describe on the actual parent decides full details.
	srv, _ = resourceServer(t, []string{actionPermit("ListNodePools"), policy}, hooks, objects...)
	w := resourceRequest(srv, "GET", "nodepools", "")
	var result struct {
		Items []struct {
			Metadata struct {
				Name   string
				Labels map[string]string
			}
		}
		Total int
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	var detailed []string
	for _, item := range result.Items {
		if item.Metadata.Labels != nil {
			detailed = append(detailed, item.Metadata.Name)
		}
	}
	if w.Code != 200 || result.Total != 2 || len(result.Items) != 2 || len(detailed) != 1 || detailed[0] != second.Name {
		t.Fatalf("wrong per-parent list details: %s", w.Body.String())
	}
}

func TestNodePoolParentFailureAbortsList(t *testing.T) {
	objects := resourceFixtures()
	hooks := interceptor.Funcs{Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*v1.Cluster); ok {
			return fmt.Errorf("parent lookup failed")
		}
		return c.Get(ctx, key, obj, opts...)
	}}
	srv, _ := resourceServer(t, []string{`permit(principal,action in Seaplane::Action::"ReadOnly",resource);`}, hooks, objects...)
	w := resourceRequest(srv, "GET", "nodepools?limit=1", "")
	if w.Code != 500 || strings.Contains(w.Body.String(), `"items"`) {
		t.Fatalf("unchecked parent yielded partial success: %s", w.Body.String())
	}
}

func TestCreatesUseValidatedCandidateLabelsAndOwnership(t *testing.T) {
	for _, tc := range []struct{ action, path, body string }{
		{"CreateCluster", "clusters", `{"metadata":{"name":"new","labels":LABELS},"spec":{"properties":{"team":"new"}}}`},
		{"CreateNodePool", "nodepools?clusterId=foreign", `{"metadata":{"name":"parent.new","labels":LABELS},"spec":{"nodePool":{"clusterName":"parent"}}}`},
		{"CreateOIDCConfig", "oidc_configs", `{"metadata":{"name":"new","labels":LABELS},"spec":{"type":"unmanaged","issuerUrl":"https://example.com","secretArn":"secret","installerRoleArn":"role"}}`},
		{"CreateDNSReservation", "dns_reservations", `{"metadata":{"name":"new","labels":LABELS}}`},
	} {
		// System labels, even syntactically invalid ones, are discarded; customer syntax errors reject the create.
		for _, check := range []struct {
			labels, grant string
			status        int
		}{
			{`{"team":"blue","seaplane.io/account-id":"222222222222","seaplane.io/cluster-namespace":"forged","seaplane.io/bad key":"bad value"}`, "blue", 201},
			{`{"team":"blue","seaplane.io/account-id":"222222222222"}`, "red", 403},
			{`{"bad key":"blue"}`, "blue", 422},
			{`{"team":"bad value"}`, "blue", 422},
		} {
			t.Run(tc.action+check.labels+check.grant, func(t *testing.T) {
				writes := 0
				policy := `permit(principal,action == Seaplane::Action::"` + tc.action + `",resource) when { resource.account == "123456789012" && resource.hasTag("team") && resource.getTag("team") == "` + check.grant + `" };`
				srv, _ := resourceServer(t, []string{policy}, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					writes++
					if obj.GetLabels()["team"] != "blue" || obj.GetLabels()["seaplane.io/account-id"] != resourceTestAccount {
						t.Errorf("lost customer labels or forged ownership: %v", obj.GetLabels())
					}
					return c.Create(ctx, obj, opts...)
				}}, resourceFixtures()...)
				w := resourceRequest(srv, "POST", tc.path, strings.Replace(tc.body, "LABELS", check.labels, 1))
				wantWrites := 0
				if check.status == 201 {
					wantWrites = 1
				}
				if w.Code != check.status || writes != wantWrites {
					t.Fatalf("status=%d writes=%d, want %d/%d: %s", w.Code, writes, check.status, wantWrites, w.Body.String())
				}
			})
		}
	}
}

func TestDeniedClusterCreateDoesNotClaimOIDC(t *testing.T) {
	objects := resourceFixtures()
	oc := objects[2].(*v1.OidcConfig)
	oc.Spec.Type = v1.OidcConfigTypeManaged
	oc.Spec.IssuerUrl = "https://example.com/config"
	writes := 0
	hooks := interceptor.Funcs{Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
		writes++
		return fmt.Errorf("unexpected OIDC write")
	}, Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
		writes++
		return fmt.Errorf("unexpected create")
	}}
	srv, store := resourceServer(t, nil, hooks, objects...)
	w := resourceRequest(srv, "POST", "clusters", `{"metadata":{"name":"new"},"spec":{"oidcConfigId":"config-uid"}}`)
	if w.Code != 403 || writes != 0 {
		t.Fatalf("denied Create claimed OIDC: %d writes=%d %s", w.Code, writes, w.Body.String())
	}
	var stored v1.OidcConfig
	if err := store.Get(context.Background(), client.ObjectKeyFromObject(oc), &stored); err != nil || stored.Labels["seaplane.io/claimed-by-cluster-uid"] != "" {
		t.Fatalf("claim changed: %+v %v", stored, err)
	}
}
