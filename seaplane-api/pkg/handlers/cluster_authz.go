package handlers

import (
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"time"

	seaplanev1alpha1 "github.com/openshift-online/rosa-seaplane-api/api/v1alpha1"
	"github.com/openshift-online/rosa-seaplane-api/seaplane-api/pkg/api"
	"github.com/openshift-online/rosa-seaplane-api/seaplane-api/pkg/authz"
	"github.com/openshift-online/rosa-seaplane-api/seaplane-api/pkg/middleware"
)

var (
	errAuthzDenied = api.APIError{Code: "AUTHZ-DENIED-001", HTTPStatus: http.StatusForbidden, Message: "Access denied"}
	errAuthzFailed = api.APIError{Code: "AUTHZ-FAILED-001", HTTPStatus: http.StatusInternalServerError, Message: "Authorization failed"}
)

func (h *ClusterHandler) clusterResource(cr *seaplanev1alpha1.Cluster) authz.Resource {
	// FleetDB's UID identifies this object incarnation independently of its name.
	return authz.Resource{
		Kind:      authz.Cluster,
		ID:        string(cr.UID),
		AccountID: cr.Labels["seaplane.io/account-id"],
		Labels:    maps.Clone(cr.Labels),
	}
}

func writeResourceAuthzError(w http.ResponseWriter, r *http.Request, attempt *authz.Attempt, operation authz.Action, err error, logger *slog.Logger) {
	stage := authz.StageEvaluation
	attrs := []any{
		"operation", operation,
		"account_id", middleware.GetAccountID(r.Context()),
		"caller_arn", middleware.GetCallerARN(r.Context()),
		"error", err,
	}
	// Failure.Error deliberately hides the diagnostics. Log their internal fields separately.
	var failure *authz.Failure
	if errors.As(err, &failure) {
		stage = failure.Stage
		attrs = append(attrs, "action", failure.Action, "cause", failure.Err, "provenance", failure.Provenance, "diagnostics", failure.Diagnostics)
	}
	attrs = append(attrs, "stage", stage)
	logger.Error("request authorization failed", attrs...)
	attempt.Finish(authz.OutcomeError, stage)
	writeAPIError(w, errAuthzFailed, logger)
}

func prepareResourceAuthz(r *http.Request, authorizer *authz.Authorizer) (*authz.Prepared, error) {
	return authorizer.Prepare(r.Context(), authz.Identity{
		AccountID: middleware.GetAccountID(r.Context()), CallerARN: middleware.GetCallerARN(r.Context()),
	}, authz.RequestContext{SourceIP: middleware.GetSourceIP(r.Context()), UserAgent: r.UserAgent(), RequestTime: time.Now()})
}

// authorizeCollection prepares one frozen policy/context snapshot for all item checks.
func authorizeCollection(w http.ResponseWriter, r *http.Request, authorizer *authz.Authorizer, logger *slog.Logger, operation authz.Action, resource authz.Resource) (*authz.Prepared, *authz.Attempt) {
	attempt := authz.DefaultMetrics.Start(operation)
	prepared, err := prepareResourceAuthz(r, authorizer)
	if err != nil {
		writeResourceAuthzError(w, r, attempt, operation, err, logger)
		return nil, nil
	}
	decision, err := prepared.Check(r.Context(), operation, resource)
	if err != nil {
		writeResourceAuthzError(w, r, attempt, operation, err, logger)
		return nil, nil
	}
	if !decision.Allowed {
		attempt.Finish(authz.OutcomeDeny, authz.StageNone)
		writeAPIError(w, errAuthzDenied, logger)
		return nil, nil
	}
	return prepared, attempt
}

// authorizeResource checks the base operation and every effective changed-field action before a write.
func authorizeResource(w http.ResponseWriter, r *http.Request, authorizer *authz.Authorizer, logger *slog.Logger, operation authz.Action, resource authz.Resource, additional ...authz.Action) bool {
	attempt := authz.DefaultMetrics.Start(operation)
	prepared, err := prepareResourceAuthz(r, authorizer)
	if err != nil {
		writeResourceAuthzError(w, r, attempt, operation, err, logger)
		return false
	}
	for _, action := range append([]authz.Action{operation}, additional...) {
		decision, err := prepared.Check(r.Context(), action, resource)
		if err != nil {
			writeResourceAuthzError(w, r, attempt, operation, err, logger)
			return false
		}
		if !decision.Allowed {
			attempt.Finish(authz.OutcomeDeny, authz.StageNone)
			writeAPIError(w, errAuthzDenied, logger)
			return false
		}
	}
	attempt.Finish(authz.OutcomeAllow, authz.StageNone)
	return true
}
