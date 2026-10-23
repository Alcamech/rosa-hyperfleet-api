package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	seaplanev1alpha1 "github.com/openshift-online/rosa-seaplane-api/api/v1alpha1"
	"github.com/openshift-online/rosa-seaplane-api/seaplane-api/pkg/api"
	"github.com/openshift-online/rosa-seaplane-api/seaplane-api/pkg/authz"
	"github.com/openshift-online/rosa-seaplane-api/seaplane-api/pkg/clients/seaplanedb"
	"k8s.io/apimachinery/pkg/types"
	labelvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errLabelBody     = APIError{Code: "LABEL-BODY-001", HTTPStatus: http.StatusBadRequest, Message: "Expected a non-empty JSON object of strings or null"}
	errLabelSyntax   = APIError{Code: "LABEL-VALIDATION-001", HTTPStatus: http.StatusUnprocessableEntity, Message: "Invalid customer label key or value"}
	errLabelNotFound = APIError{Code: "LABEL-NOTFOUND-001", HTTPStatus: http.StatusNotFound, Message: "Resource not found"}
	errLabelFailed   = APIError{Code: "LABEL-FAILED-001", HTTPStatus: http.StatusInternalServerError, Message: "Label operation failed"}
)

// Customer creates discard system labels; validate only the customer-controlled entries.
func validateCreateLabels(w http.ResponseWriter, labels map[string]string, logger *slog.Logger) bool {
	for key, value := range labels {
		if strings.HasPrefix(key, "seaplane.io/") {
			continue
		}
		if !isValidLabel(key, &value) {
			writeAPIError(w, errLabelSyntax, logger)
			return false
		}
	}
	return true
}

func isValidLabel(key string, value *string) bool {
	return !strings.HasPrefix(key, "seaplane.io/") && len(labelvalidation.IsQualifiedName(key)) == 0 && (value == nil || len(labelvalidation.IsValidLabelValue(*value)) == 0)
}

// applyLabelPatch preserves every key outside the submitted operations.
func applyLabelPatch(stored map[string]string, patch map[string]*string) map[string]string {
	labels := maps.Clone(stored)
	if labels == nil {
		labels = map[string]string{}
	}
	for key, value := range patch {
		if value == nil {
			delete(labels, key)
		} else {
			labels[key] = *value
		}
	}
	return labels
}

// Each retry reapplies only submitted keys to a freshly loaded and authorized snapshot.
func resourceLabels[T client.Object](w http.ResponseWriter, r *http.Request, authorizer *authz.Authorizer, logger *slog.Logger, load func(context.Context, string) (T, error), save func(context.Context, T) error, resource func(context.Context, T) (authz.Resource, error)) {
	var patch map[string]*string
	operation := authz.ListLabelsForResource
	var additional []authz.Action
	if r.Method == http.MethodPatch {
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&patch); err != nil || len(patch) == 0 {
			writeAPIError(w, errLabelBody, logger)
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			writeAPIError(w, errLabelBody, logger)
			return
		}
		hasSet, hasRemove := false, false
		for key, value := range patch {
			if !isValidLabel(key, value) {
				writeAPIError(w, errLabelSyntax, logger)
				return
			}
			if value == nil {
				hasRemove = true
			} else {
				hasSet = true
			}
		}
		operation = authz.UnlabelResource
		if hasSet {
			operation = authz.LabelResource
		}
		if hasSet && hasRemove {
			additional = []authz.Action{authz.UnlabelResource}
		}
	}
	r, finish := withAuthzOutcome(r)
	defer finish()
	var uid types.UID
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj, err := load(r.Context(), mux.Vars(r)["id"])
		if err != nil {
			if seaplanedb.IsNotFound(err) {
				if uid != "" {
					writeAPIError(w, ErrResourceConflict, logger)
				} else {
					writeAPIError(w, errLabelNotFound, logger)
				}
			} else {
				logger.Error("failed to load labels", "error", err)
				writeAPIError(w, errLabelFailed, logger)
			}
			return errResponded
		}
		if uid != "" && obj.GetUID() != uid {
			writeAPIError(w, ErrResourceConflict, logger)
			return errResponded
		}
		uid = obj.GetUID()
		snapshot, err := resource(r.Context(), obj)
		if err != nil {
			writeResourceAuthzError(w, r, startResourceAuthz(r, operation), operation, err, logger)
			return errResponded
		}
		if !authorizeResource(w, r, authorizer, logger, operation, snapshot, additional...) {
			return errResponded
		}
		if r.Method == http.MethodGet {
			labels := obj.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			if err := api.Write(w, http.StatusOK, labels); err != nil {
				logger.Error("failed to write labels", "error", err)
			}
			return nil
		}
		labels := applyLabelPatch(obj.GetLabels(), patch)
		if maps.Equal(labels, obj.GetLabels()) {
			return nil
		}
		obj.SetLabels(labels)
		if err := save(r.Context(), obj); err != nil {
			if seaplanedb.IsConflict(err) {
				return err
			}
			if seaplanedb.IsNotFound(err) {
				writeAPIError(w, ErrResourceConflict, logger)
				return errResponded
			}
			logger.Error("failed to save labels", "error", err)
			writeAPIError(w, errLabelFailed, logger)
			return errResponded
		}
		return nil
	})
	if err == nil && r.Method == http.MethodPatch {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if seaplanedb.IsConflict(err) {
		writeAPIError(w, ErrResourceConflict, logger)
	}
}

func (h *ClusterHandler) Labels(w http.ResponseWriter, r *http.Request) {
	resourceLabels(w, r, h.authorizer, h.logger, h.db.GetCluster, h.db.UpdateCluster, func(_ context.Context, obj *seaplanev1alpha1.Cluster) (authz.Resource, error) {
		return h.clusterResource(obj), nil
	})
}
func (h *NodePoolHandler) Labels(w http.ResponseWriter, r *http.Request) {
	resourceLabels(w, r, h.authorizer, h.logger, h.db.GetNodePool, h.db.UpdateNodePool, h.nodePoolResource)
}
func (h *OidcConfigHandler) Labels(w http.ResponseWriter, r *http.Request) {
	resourceLabels(w, r, h.authorizer, h.logger, h.db.GetOidcConfig, h.db.UpdateOidcConfigObject, func(_ context.Context, obj *seaplanev1alpha1.OidcConfig) (authz.Resource, error) {
		return h.oidcConfigResource(obj), nil
	})
}
func (h *DNSReservationHandler) Labels(w http.ResponseWriter, r *http.Request) {
	resourceLabels(w, r, h.authorizer, h.logger, func(ctx context.Context, name string) (*seaplanev1alpha1.DNSReservation, error) {
		return h.db.GetDNSReservation(ctx, name)
	}, h.db.UpdateDNSReservation, func(_ context.Context, obj *seaplanev1alpha1.DNSReservation) (authz.Resource, error) {
		return h.dnsReservationResource(obj), nil
	})
}
