package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestResourceLabels(t *testing.T) {
	for _, route := range []struct {
		path  string
		index int
	}{{"clusters/parent", 0}, {"nodepools/parent.workers", 1}, {"oidc_configs/config", 2}, {"dns_reservations/zone", 3}} {
		for _, tc := range []struct {
			name, grant, method, body string
			status, writes            int
		}{
			{"read", "ReadOnly", "GET", "", 200, 0},
			{"read cannot write", "ReadOnly", "PATCH", `{"team":"red"}`, 403, 0},
			{"write cannot read", "LabelAdmin", "GET", "", 403, 0},
			{"set", "LabelResource", "PATCH", `{"team":"red","example.com/empty":""}`, 204, 1},
			{"remove", "UnlabelResource", "PATCH", `{"team":null}`, 204, 1},
			{"mixed lacks remove", "LabelResource", "PATCH", `{"new":"value","team":null}`, 403, 0},
			{"mixed lacks set", "UnlabelResource", "PATCH", `{"new":"value","team":null}`, 403, 0},
			{"mixed", "LabelAdmin", "PATCH", `{"new":"value","team":null}`, 204, 1},
			{"unchanged", "LabelResource", "PATCH", `{"team":"blue"}`, 204, 0},
			{"absent", "UnlabelResource", "PATCH", `{"absent":null}`, 204, 0},
			{"unchanged needs set", "UnlabelResource", "PATCH", `{"team":"blue"}`, 403, 0},
			{"absent needs remove", "LabelResource", "PATCH", `{"absent":null}`, 403, 0},
			{"system", "AllActions", "PATCH", `{"new":"value","seaplane.io/absent":null}`, 422, 0},
			{"syntax", "AllActions", "PATCH", `{"bad key":"value"}`, 422, 0},
			{"value syntax", "AllActions", "PATCH", `{"team":"bad value"}`, 422, 0},
			{"empty", "AllActions", "PATCH", `{}`, 400, 0},
			{"null", "AllActions", "PATCH", `null`, 400, 0},
			{"array", "AllActions", "PATCH", `[]`, 400, 0},
			{"number", "AllActions", "PATCH", `{"team":1}`, 400, 0},
			{"trailing", "AllActions", "PATCH", `{"team":"red"} {}`, 400, 0},
		} {
			// Shared validation/authorization semantics run once; other kinds check wiring and preservation.
			if route.index != 0 && tc.name != "read" && tc.name != "set" && tc.name != "remove" && tc.name != "mixed" && tc.name != "system" {
				continue
			}
			t.Run(route.path+"/"+tc.name, func(t *testing.T) {
				objects := resourceFixtures()
				original := objects[route.index].DeepCopyObject().(client.Object)
				writes := 0
				hooks := interceptor.Funcs{Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					writes++
					return c.Update(ctx, obj, opts...)
				}}
				policy := fmt.Sprintf(`permit(principal, action in Seaplane::Action::"%s", resource);`, tc.grant)
				srv, store := resourceServer(t, []string{policy}, hooks, objects...)
				w := resourceRequest(srv, tc.method, route.path+"/labels", tc.body)
				if w.Code != tc.status || writes != tc.writes {
					t.Fatalf("status=%d writes=%d body=%s, want %d/%d", w.Code, writes, w.Body.String(), tc.status, tc.writes)
				}
				if w.Code == 204 && w.Body.Len() != 0 {
					t.Fatal("mutation returned a body")
				}
				stored := original.DeepCopyObject().(client.Object)
				if err := store.Get(context.Background(), client.ObjectKeyFromObject(original), stored); err != nil {
					t.Fatal(err)
				}
				if tc.method == "GET" && w.Code == 200 {
					var labels map[string]string
					if err := json.Unmarshal(w.Body.Bytes(), &labels); err != nil || !reflect.DeepEqual(labels, original.GetAnnotations()) {
						t.Fatalf("labels=%v error=%v", labels, err)
					}
				}
				expected := original.DeepCopyObject().(client.Object)
				if tc.writes > 0 {
					var patch map[string]*string
					if err := json.Unmarshal([]byte(tc.body), &patch); err != nil {
						t.Fatal(err)
					}
					tags := maps.Clone(expected.GetAnnotations())
					for key, value := range patch {
						if value == nil {
							delete(tags, key)
						} else {
							tags[key] = *value
						}
					}
					if len(tags) == 0 {
						tags = nil // the store omits an empty map
					}
					expected.SetAnnotations(tags)
				}
				expected.SetResourceVersion(stored.GetResourceVersion())
				if !reflect.DeepEqual(expected, stored) {
					t.Fatalf("unexpected stored change: got %#v want %#v", stored, expected)
				}
			})
		}
	}
}
