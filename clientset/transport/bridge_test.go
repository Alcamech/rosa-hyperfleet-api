/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package transport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func newAdapter() *Adapter {
	return NewAdapter(http.DefaultTransport)
}

func postRequest(rawURL, body string) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, rawURL, io.NopCloser(strings.NewReader(body)))
	return req
}

func putRequest(rawURL, body string) *http.Request {
	req, _ := http.NewRequest(http.MethodPut, rawURL, io.NopCloser(strings.NewReader(body)))
	return req
}

func getRequest(rawURL string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	return req
}

func readRequestBody(req *http.Request) map[string]json.RawMessage {
	if req.Body == nil {
		return nil
	}
	b, _ := io.ReadAll(req.Body)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)
	return m
}

func readResponseBody(resp *http.Response) map[string]json.RawMessage {
	b, _ := io.ReadAll(resp.Body)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)
	return m
}

func jsonStr(v json.RawMessage) string { return string(v) }

func assertField(t *testing.T, m map[string]json.RawMessage, key, want string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Errorf("field %q missing", key)
		return
	}
	if jsonStr(v) != want {
		t.Errorf("field %q = %s, want %s", key, jsonStr(v), want)
	}
}

func assertNoField(t *testing.T, m map[string]json.RawMessage, key string) {
	t.Helper()
	if _, ok := m[key]; ok {
		t.Errorf("field %q should be absent", key)
	}
}

// mustAdaptRequest calls adaptRequest and fails the test if it returns an error.
// The resource type is derived from the request URL path.
func mustAdaptRequest(t *testing.T, a *Adapter, req *http.Request) *http.Request {
	t.Helper()
	mappings := a.mappings[resourceFromPath(req.URL.Path, a.mappings)]
	out, err := a.adaptRequest(req, mappings)
	if err != nil {
		t.Fatalf("adaptRequest: unexpected error: %v", err)
	}
	return out
}

// mustAdaptResponse calls adaptResponse and fails the test if it returns an error.
// resource is the lowercase plural resource name (e.g. "clusters", "nodepools", or ""
// for tests that do not depend on field mappings).
func mustAdaptResponse(t *testing.T, a *Adapter, resource string, resp *http.Response) *http.Response {
	t.Helper()
	out, err := a.adaptResponse(resp, a.mappings[resource])
	if err != nil {
		t.Fatalf("adaptResponse: unexpected error: %v", err)
	}
	return out
}

// errReader returns the given error on Read.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// partialErrReader returns data and err in a single Read call, simulating a
// reader that delivers bytes and a read error simultaneously.
type partialErrReader struct {
	data []byte
	err  error
}

func (r partialErrReader) Read(p []byte) (int, error) {
	return copy(p, r.data), r.err
}

// errCloser wraps a reader and returns the given error on Close.
type errCloser struct {
	io.Reader
	err error
}

func (e errCloser) Close() error { return e.err }

// adaptRequest tests

func TestAdaptRequest_MetadataFlattenedToTopLevel(t *testing.T) {
	a := newAdapter()
	req := putRequest("https://example.com/api/v0/clusters/uid-1",
		`{"metadata":{"name":"my-cluster","uid":"uid-1","resourceVersion":"42","generation":3},"spec":{"foo":"bar"}}`)

	adapted := mustAdaptRequest(t, a, req)
	m := readRequestBody(adapted)

	assertField(t, m, "name", `"my-cluster"`)
	assertField(t, m, "id", `"uid-1"`)
	assertField(t, m, "resource_version", `"42"`)
	assertField(t, m, "generation", `3`)
	assertNoField(t, m, "metadata")
}

func TestAdaptRequest_SpecPassesThroughUnchanged(t *testing.T) {
	a := newAdapter()
	req := putRequest("https://example.com/api/v0/clusters/uid-1",
		`{"metadata":{"name":"c"},"spec":{"replicas":3}}`)

	adapted := mustAdaptRequest(t, a, req)
	m := readRequestBody(adapted)

	assertField(t, m, "spec", `{"replicas":3}`)
}

func TestAdaptRequest_ClusterIDInjectedOnPOSTWithNamespace(t *testing.T) {
	a := newAdapter()
	req := postRequest("https://example.com/api/v0/namespaces/cluster-uuid-123/nodepools",
		`{"metadata":{"name":"np1"},"spec":{}}`)

	adapted := mustAdaptRequest(t, a, req)
	m := readRequestBody(adapted)

	assertField(t, m, "cluster_id", `"cluster-uuid-123"`)
}

func TestAdaptRequest_ClusterIDNotInjectedOnClusterPOST(t *testing.T) {
	a := newAdapter()
	// Cluster creation: namespace is account ID, not a parent cluster ID.
	req := postRequest("https://example.com/api/v0/namespaces/123456789012/clusters",
		`{"metadata":{"name":"my-cluster"},"spec":{}}`)

	adapted := mustAdaptRequest(t, a, req)
	m := readRequestBody(adapted)

	assertNoField(t, m, "cluster_id")
}

func TestAdaptRequest_ClusterIDNotOverwrittenIfAlreadyPresent(t *testing.T) {
	a := newAdapter()
	req := postRequest("https://example.com/api/v0/namespaces/cluster-uuid-123/nodepools",
		`{"metadata":{"name":"np1"},"cluster_id":"existing-id","spec":{}}`)

	adapted := mustAdaptRequest(t, a, req)
	m := readRequestBody(adapted)

	assertField(t, m, "cluster_id", `"existing-id"`)
}

func TestAdaptRequest_ClusterIDNotInjectedOnNonPOST(t *testing.T) {
	a := newAdapter()
	req := putRequest("https://example.com/api/v0/namespaces/cluster-uuid-123/nodepools/np1",
		`{"metadata":{"name":"np1"},"spec":{}}`)

	adapted := mustAdaptRequest(t, a, req)
	m := readRequestBody(adapted)

	assertNoField(t, m, "cluster_id")
}

func TestAdaptRequest_NilBodyReturnedUnchanged(t *testing.T) {
	a := newAdapter()
	req := getRequest("https://example.com/api/v0/clusters")

	adapted := mustAdaptRequest(t, a, req)
	if adapted.Body != nil {
		t.Error("expected nil body to remain nil")
	}
}

func TestAdaptRequest_NoMetadataPassesThrough(t *testing.T) {
	a := newAdapter()
	original := `{"spec":{"foo":"bar"}}`
	req := putRequest("https://example.com/api/v0/clusters/id", original)

	adapted := mustAdaptRequest(t, a, req)
	b, _ := io.ReadAll(adapted.Body)
	if string(b) != original {
		t.Errorf("body = %q, want %q", string(b), original)
	}
}

func TestAdaptRequest_InvalidJSONPassesThrough(t *testing.T) {
	a := newAdapter()
	original := `not json`
	req := putRequest("https://example.com/api/v0/clusters/id", original)

	adapted := mustAdaptRequest(t, a, req)
	b, _ := io.ReadAll(adapted.Body)
	if string(b) != original {
		t.Errorf("body = %q, want %q", string(b), original)
	}
}

func TestAdaptRequest_ReadErrorPropagated(t *testing.T) {
	a := newAdapter()
	req, _ := http.NewRequest(http.MethodPut, "https://example.com/api/v0/clusters/id",
		io.NopCloser(errReader{err: errors.New("read failure")}))

	if _, err := a.adaptRequest(req, nil); err == nil {
		t.Error("expected error when body read fails")
	}
}

func TestAdaptRequest_CloseErrorPropagated(t *testing.T) {
	a := newAdapter()
	req, _ := http.NewRequest(http.MethodPut, "https://example.com/api/v0/clusters/id",
		errCloser{Reader: strings.NewReader(`{"metadata":{"name":"c"},"spec":{}}`), err: errors.New("close failure")})

	if _, err := a.adaptRequest(req, nil); err == nil {
		t.Error("expected error when body close fails")
	}
}

// adaptResponse tests

func TestAdaptResponse_SingleItemLiftedIntoMetadata(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 200,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"uid-1","name":"my-cluster","resource_version":"5","generation":2,"spec":{"foo":"bar"}}`)),
		Header: make(http.Header),
	}

	out := mustAdaptResponse(t, a, "clusters", resp)
	m := readResponseBody(out)

	assertNoField(t, m, "id")
	assertNoField(t, m, "name")
	assertNoField(t, m, "resource_version")

	var meta map[string]json.RawMessage
	_ = json.Unmarshal(m["metadata"], &meta)
	assertField(t, meta, "uid", `"uid-1"`)
	assertField(t, meta, "name", `"my-cluster"`)
	assertField(t, meta, "resourceVersion", `"5"`)
	assertField(t, meta, "generation", `2`)
	assertField(t, m, "spec", `{"foo":"bar"}`)
}

func TestAdaptResponse_ListItemsAdapted(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 200,
		Body: io.NopCloser(strings.NewReader(
			`{"items":[{"id":"uid-1","name":"c1","spec":{}},{"id":"uid-2","name":"c2","spec":{}}]}`)),
		Header: make(http.Header),
	}

	out := mustAdaptResponse(t, a, "clusters", resp)
	m := readResponseBody(out)

	var items []map[string]json.RawMessage
	_ = json.Unmarshal(m["items"], &items)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for i, item := range items {
		if _, ok := item["metadata"]; !ok {
			t.Errorf("items[%d] missing metadata", i)
		}
		assertNoField(t, item, "id")
	}
}

func errorResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func readMetav1Status(t *testing.T, resp *http.Response) map[string]json.RawMessage {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("body is not JSON: %v — body: %s", err, b)
	}
	return m
}

func TestAdaptResponse_NonPlatformAPIErrorPassesThrough(t *testing.T) {
	a := newAdapter()
	original := `{"message":"not found"}`
	resp := &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader(original)),
		Header:     make(http.Header),
	}

	out := mustAdaptResponse(t, a, "", resp)
	b, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(b) != original {
		t.Errorf("body changed on non-platform-api error response: %s", b)
	}
}

// adaptErrorResponse tests

func TestAdaptErrorResponse_PlatformAPIErrorTranslatedToMetav1Status(t *testing.T) {
	resp := errorResponse(400, `{"kind":"Error","code":"CLUSTERS-MGMT-001","reason":"Invalid request body"}`)
	out, err := adaptErrorResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := readMetav1Status(t, out)
	assertField(t, m, "apiVersion", `"v1"`)
	assertField(t, m, "kind", `"Status"`)
	assertField(t, m, "status", `"Failure"`)
	assertField(t, m, "message", `"CLUSTERS-MGMT-001: Invalid request body"`)
	assertField(t, m, "reason", `"BadRequest"`)
	assertField(t, m, "code", `400`)
	if out.ContentLength != int64(len(mustMarshal(t, m))) {
		t.Errorf("ContentLength not updated")
	}
}

func TestAdaptErrorResponse_EmptyCodeUsesReasonOnly(t *testing.T) {
	resp := errorResponse(404, `{"kind":"Error","code":"","reason":"cluster not found"}`)
	out, err := adaptErrorResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := readMetav1Status(t, out)
	assertField(t, m, "message", `"cluster not found"`)
}

func TestAdaptErrorResponse_NonJSONBodyPassesThrough(t *testing.T) {
	original := `not json at all`
	resp := errorResponse(500, original)
	out, err := adaptErrorResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(b) != original {
		t.Errorf("body changed: %s", b)
	}
}

func TestAdaptErrorResponse_JSONWithoutKindErrorPassesThrough(t *testing.T) {
	original := `{"kind":"Cluster","name":"my-cluster"}`
	resp := errorResponse(400, original)
	out, err := adaptErrorResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(b) != original {
		t.Errorf("body changed: %s", b)
	}
}

func TestAdaptErrorResponse_HTTPStatusMappedToReason(t *testing.T) {
	cases := []struct {
		code   int
		reason string
	}{
		{http.StatusBadRequest, "BadRequest"},
		{http.StatusUnauthorized, "Unauthorized"},
		{http.StatusForbidden, "Forbidden"},
		{http.StatusNotFound, "NotFound"},
		{http.StatusConflict, "Conflict"},
		{http.StatusUnprocessableEntity, "Invalid"},
		{http.StatusTooManyRequests, "TooManyRequests"},
		{http.StatusInternalServerError, "InternalError"},
		{http.StatusServiceUnavailable, "ServiceUnavailable"},
		{http.StatusGatewayTimeout, "Timeout"},
		{http.StatusTeapot, "Unknown"},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			resp := errorResponse(tc.code, `{"kind":"Error","code":"X","reason":"Y"}`)
			out, err := adaptErrorResponse(resp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := readMetav1Status(t, out)
			assertField(t, m, "reason", string(mustMarshal(t, tc.reason)))
		})
	}
}

func TestAdaptErrorResponse_PartialReadErrorPropagated(t *testing.T) {
	// partialErrReader delivers bytes and an error in the same Read call.
	// Previously the code restored the partial body and returned nil; it must now propagate the error.
	resp := &http.Response{
		StatusCode: 403,
		Body:       io.NopCloser(partialErrReader{data: []byte(`{"kind":"Error"`), err: errors.New("network interrupted")}),
		Header:     make(http.Header),
	}
	if _, err := adaptErrorResponse(resp); err == nil {
		t.Error("expected error when body read fails partway through")
	}
}

func TestAdaptErrorResponse_CloseErrorPropagated(t *testing.T) {
	resp := &http.Response{
		StatusCode: 403,
		Body:       errCloser{Reader: strings.NewReader(`{"kind":"Error","code":"X","reason":"Y"}`), err: errors.New("close failure")},
		Header:     make(http.Header),
	}
	if _, err := adaptErrorResponse(resp); err == nil {
		t.Error("expected error when body close fails")
	}
}

func TestAdaptResponse_PlatformAPIErrorSurfacedAsMetav1Status(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 409,
		Body:       io.NopCloser(strings.NewReader(`{"kind":"Error","code":"CLUSTERS-MGMT-409","reason":"cluster already exists"}`)),
		Header:     make(http.Header),
	}

	out := mustAdaptResponse(t, a, "", resp)
	m := readMetav1Status(t, out)
	assertField(t, m, "kind", `"Status"`)
	assertField(t, m, "status", `"Failure"`)
	assertField(t, m, "reason", `"Conflict"`)
	assertField(t, m, "message", `"CLUSTERS-MGMT-409: cluster already exists"`)
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func TestAdaptResponse_NoWireFieldsPassesThrough(t *testing.T) {
	a := newAdapter()
	original := `{"some_other_field":"value"}`
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(original)),
		Header:     make(http.Header),
	}

	out := mustAdaptResponse(t, a, "", resp)
	b, _ := io.ReadAll(out.Body)
	if string(b) != original {
		t.Errorf("body unexpectedly changed: %s", b)
	}
}

func TestAdaptResponse_MalformedListItemPassesThroughUnchanged(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`{"items":["not an object"]}`)),
		Header:     make(http.Header),
	}

	out := mustAdaptResponse(t, a, "", resp)
	m := readResponseBody(out)

	var items []json.RawMessage
	_ = json.Unmarshal(m["items"], &items)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
}

func TestAdaptResponse_NodepoolClusterIDMappedToNamespace(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 200,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"np-uid","name":"my-np","cluster_id":"cluster-uid","resource_version":"1","generation":1,"spec":{}}`)),
		Header: make(http.Header),
	}

	out := mustAdaptResponse(t, a, "nodepools", resp)
	m := readResponseBody(out)

	var meta map[string]json.RawMessage
	_ = json.Unmarshal(m["metadata"], &meta)
	assertField(t, meta, "namespace", `"cluster-uid"`)
	assertNoField(t, m, "cluster_id")
}

func TestAdaptResponse_ClusterDoesNotMapClusterIDToNamespace(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 200,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"cluster-uid","name":"my-cluster","resource_version":"1","generation":1,"spec":{}}`)),
		Header: make(http.Header),
	}

	out := mustAdaptResponse(t, a, "clusters", resp)
	m := readResponseBody(out)

	var meta map[string]json.RawMessage
	_ = json.Unmarshal(m["metadata"], &meta)
	if _, ok := meta["namespace"]; ok {
		t.Error("clusters mapping should not produce a namespace field")
	}
}

func TestAdaptResponse_ReadErrorPropagated(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(errReader{err: errors.New("read failure")}),
		Header:     make(http.Header),
	}

	if _, err := a.adaptResponse(resp, nil); err == nil {
		t.Error("expected error when response body read fails")
	}
}

func TestAdaptResponse_CloseErrorPropagated(t *testing.T) {
	a := newAdapter()
	resp := &http.Response{
		StatusCode: 200,
		Body:       errCloser{Reader: strings.NewReader(`{"id":"x","name":"y","spec":{}}`), err: errors.New("close failure")},
		Header:     make(http.Header),
	}

	if _, err := a.adaptResponse(resp, nil); err == nil {
		t.Error("expected error when response body close fails")
	}
}

// adaptListQuery tests

func TestAdaptListQuery_NumericContinueRewrittenToOffset(t *testing.T) {
	a := newAdapter()
	req := getRequest("https://example.com/api/v0/clusters?continue=50")

	adapted := a.adaptListQuery(req)
	q := adapted.URL.Query()
	if q.Get("offset") != "50" {
		t.Errorf("offset = %q, want 50", q.Get("offset"))
	}
	if q.Get("continue") != "" {
		t.Errorf("continue should be removed, got %q", q.Get("continue"))
	}
}

func TestAdaptListQuery_NonNumericContinuePassesThrough(t *testing.T) {
	a := newAdapter()
	req := getRequest("https://example.com/api/v0/clusters?continue=cursor-token")

	adapted := a.adaptListQuery(req)
	q := adapted.URL.Query()
	if q.Get("continue") != "cursor-token" {
		t.Errorf("continue = %q, want cursor-token", q.Get("continue"))
	}
	if q.Get("offset") != "" {
		t.Errorf("offset should not be set: %q", q.Get("offset"))
	}
}

func TestAdaptListQuery_NoContinuePassesThrough(t *testing.T) {
	a := newAdapter()
	req := getRequest("https://example.com/api/v0/clusters?limit=10")

	adapted := a.adaptListQuery(req)
	if adapted.URL.Query().Get("offset") != "" {
		t.Error("offset should not be set when continue is absent")
	}
}

func TestAdaptListQuery_NonGETNotRewritten(t *testing.T) {
	a := newAdapter()
	req := postRequest("https://example.com/api/v0/clusters?continue=50", `{}`)

	adapted := a.adaptListQuery(req)
	q := adapted.URL.Query()
	if q.Get("offset") != "" {
		t.Error("offset should not be set on non-GET request")
	}
	if q.Get("continue") != "50" {
		t.Errorf("continue should be preserved on non-GET: got %q", q.Get("continue"))
	}
}
