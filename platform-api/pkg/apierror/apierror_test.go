package apierror_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/apierror"
)

var base = apierror.APIError{
	Code:       "TEST-001",
	HTTPStatus: http.StatusBadRequest,
	Message:    "something went wrong",
}

// structuredError has exported fields so it marshals to non-empty JSON.
type structuredError struct {
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

func (e *structuredError) Error() string { return e.Detail }

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func write(def apierror.APIError) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	apierror.Write(w, def)
	return w
}

// --- WithErrors ---

func TestWithErrors_SetsErrors(t *testing.T) {
	payload := []string{"a", "b"}
	got := base.WithErrors(payload)
	if got.Errors == nil {
		t.Fatal("expected Errors to be set")
	}
}

func TestWithErrors_DoesNotMutateBase(t *testing.T) {
	_ = base.WithErrors("x")
	if base.Errors != nil {
		t.Fatal("WithErrors must not mutate the receiver")
	}
}

// --- WithReason ---

func TestWithReason_AppliesTemplate(t *testing.T) {
	e := apierror.APIError{Code: "X", HTTPStatus: 400, Message: "m", Reason: "hello %s"}
	got := e.WithReason("world")
	if got.Errors == nil {
		t.Fatal("expected Errors to be set")
	}
	if got.Errors.(error).Error() != "hello world" {
		t.Fatalf("unexpected reason: %v", got.Errors)
	}
}

func TestWithReason_WrapsErrorWithW(t *testing.T) {
	sentinel := errors.New("sentinel")
	e := apierror.APIError{Code: "X", HTTPStatus: 500, Message: "m", Reason: "%w"}
	got := e.WithReason(sentinel)
	if !errors.Is(got.Errors.(error), sentinel) {
		t.Fatal("expected error chain to be preserved via %w")
	}
}

func TestWithReason_PanicsWithoutTemplate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when Reason is empty")
		}
	}()
	base.WithReason("arg")
}

func TestWithReason_DoesNotMutateBase(t *testing.T) {
	e := apierror.APIError{Code: "X", HTTPStatus: 400, Message: "m", Reason: "%s"}
	_ = e.WithReason("x")
	if e.Errors != nil {
		t.Fatal("WithReason must not mutate the receiver")
	}
}

// --- Write: HTTP envelope ---

func TestWrite_StatusCode(t *testing.T) {
	w := write(apierror.APIError{Code: "X", HTTPStatus: http.StatusNotFound, Message: "m"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWrite_ContentType(t *testing.T) {
	w := write(base)
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestWrite_KindIsError(t *testing.T) {
	w := write(base)
	resp := decode(t, w)
	if resp["kind"] != "Error" {
		t.Fatalf("expected kind=Error, got %v", resp["kind"])
	}
}

func TestWrite_CodeAndReason(t *testing.T) {
	w := write(base)
	resp := decode(t, w)
	if resp["code"] != "TEST-001" {
		t.Fatalf("unexpected code: %v", resp["code"])
	}
	if resp["reason"] != "something went wrong" {
		t.Fatalf("unexpected reason: %v", resp["reason"])
	}
}

// --- Write: plain error (no exported fields) ---

func TestWrite_PlainError_ReasonFromError(t *testing.T) {
	e := apierror.APIError{Code: "TEST-001", HTTPStatus: http.StatusNotFound, Message: "not found", Reason: "cluster %q not found"}
	w := write(e.WithReason("abc"))
	resp := decode(t, w)
	if resp["reason"] != `cluster "abc" not found` {
		t.Fatalf("unexpected reason: %v", resp["reason"])
	}
}

func TestWrite_PlainError_ErrorsFieldSuppressed(t *testing.T) {
	e := apierror.APIError{Code: "TEST-001", HTTPStatus: http.StatusBadRequest, Message: "bad", Reason: "%w"}
	w := write(e.WithReason(errors.New("oops")))
	resp := decode(t, w)
	if _, ok := resp["errors"]; ok {
		t.Fatal("errors field must be suppressed for plain errors")
	}
}

// --- Write: structured error (exported fields) ---

func TestWrite_StructuredError_ReasonIsStatic(t *testing.T) {
	def := base.WithErrors(&structuredError{Field: "foo", Detail: "too long"})
	w := write(def)
	resp := decode(t, w)
	if resp["reason"] != "something went wrong" {
		t.Fatalf("expected static reason, got %v", resp["reason"])
	}
}

func TestWrite_StructuredError_ErrorsFieldPresent(t *testing.T) {
	def := base.WithErrors(&structuredError{Field: "foo", Detail: "too long"})
	w := write(def)
	resp := decode(t, w)
	if resp["errors"] == nil {
		t.Fatal("expected errors field to be present for structured errors")
	}
	errs := resp["errors"].(map[string]any)
	if errs["field"] != "foo" {
		t.Fatalf("unexpected errors.field: %v", errs["field"])
	}
}

// --- Write: no errors ---

func TestWrite_NoErrors_NoErrorsField(t *testing.T) {
	w := write(base)
	resp := decode(t, w)
	if _, ok := resp["errors"]; ok {
		t.Fatal("errors field must be absent when not set")
	}
}

// --- Write: full response format ---

func TestWrite_ResponseFormat(t *testing.T) {
	cases := []struct {
		name        string
		def         apierror.APIError
		wantStatus  int
		wantKind    string
		wantCode    string
		wantReason  string
		wantErrors  any    // nil means field must be absent
		forbidden   []string // keys that must not appear in the response
	}{
		{
			name:       "static message no errors",
			def:        apierror.APIError{Code: "A-001", HTTPStatus: http.StatusBadRequest, Message: "bad request"},
			wantStatus: http.StatusBadRequest,
			wantKind:   "Error",
			wantCode:   "A-001",
			wantReason: "bad request",
			forbidden:  []string{"HTTPStatus", "http_status", "Reason", "Format"},
		},
		{
			name:       "plain error derives reason and suppresses errors field",
			def:        apierror.APIError{Code: "A-002", HTTPStatus: http.StatusNotFound, Message: "default", Reason: "item %q not found"}.WithReason("xyz"),
			wantStatus: http.StatusNotFound,
			wantKind:   "Error",
			wantCode:   "A-002",
			wantReason: `item "xyz" not found`,
			forbidden:  []string{"HTTPStatus", "http_status", "Reason", "Format"},
		},
		{
			name:       "structured error keeps static reason and exposes errors",
			def:        apierror.APIError{Code: "A-003", HTTPStatus: http.StatusUnprocessableEntity, Message: "validation failed"}.WithErrors(&structuredError{Field: "name", Detail: "required"}),
			wantStatus: http.StatusUnprocessableEntity,
			wantKind:   "Error",
			wantCode:   "A-003",
			wantReason: "validation failed",
			wantErrors: map[string]any{"field": "name", "detail": "required"},
			forbidden:  []string{"HTTPStatus", "http_status", "Reason", "Format"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := write(tc.def)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantStatus)
			}

			resp := decode(t, w)

			if resp["kind"] != tc.wantKind {
				t.Errorf("kind: got %v, want %q", resp["kind"], tc.wantKind)
			}
			if resp["code"] != tc.wantCode {
				t.Errorf("code: got %v, want %q", resp["code"], tc.wantCode)
			}
			if resp["reason"] != tc.wantReason {
				t.Errorf("reason: got %v, want %q", resp["reason"], tc.wantReason)
			}

			if tc.wantErrors == nil {
				if _, ok := resp["errors"]; ok {
					t.Errorf("errors: expected absent, got %v", resp["errors"])
				}
			} else {
				if resp["errors"] == nil {
					t.Error("errors: expected present, got absent")
				}
			}

			for _, key := range tc.forbidden {
				if _, ok := resp[key]; ok {
					t.Errorf("internal field %q must not appear in response", key)
				}
			}
		})
	}
}
