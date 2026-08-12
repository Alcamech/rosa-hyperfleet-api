package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/internal/codegen/featuregate"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/clients/hyperfleetdb"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/middleware"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/types"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/validation"
)

// ClusterHandler handles cluster-related HTTP requests
type ClusterHandler struct {
	db                       *hyperfleetdb.Client
	oidcIssuerBaseURL        string
	defaultClusterExpiration time.Duration
	validator                *validation.FieldValidator
	logger                   *slog.Logger
	generateID               func() string
}

// NewClusterHandler creates a new cluster handler
func NewClusterHandler(db *hyperfleetdb.Client, oidcIssuerBaseURL string, defaultClusterExpiration time.Duration, logger *slog.Logger) *ClusterHandler {
	return &ClusterHandler{
		db:                       db,
		oidcIssuerBaseURL:        oidcIssuerBaseURL,
		defaultClusterExpiration: defaultClusterExpiration,
		validator:                validation.NewFieldValidator(),
		logger:                   logger,
		generateID:               func() string { return uuid.New().String() },
	}
}

// List handles GET /api/v0/clusters
func (h *ClusterHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	h.logger.Info("listing clusters", "account_id", accountID, "limit", limit, "offset", offset)

	list, err := h.db.ListClusters(ctx, accountID)
	if err != nil {
		h.logger.Error("failed to list clusters", "error", err, "account_id", accountID)
		writeAPIError(w, ErrClusterList)
		return
	}

	clusters := make([]*types.Cluster, 0, len(list.Items))
	for i := range list.Items {
		clusters = append(clusters, hyperfleetdb.ClusterCRToPlatform(&list.Items[i]))
	}

	total := len(clusters)

	// Apply offset/limit pagination in-memory.
	if offset >= len(clusters) {
		clusters = []*types.Cluster{}
	} else {
		end := min(offset+limit, len(clusters))
		clusters = clusters[offset:end]
	}

	response := map[string]any{
		"items":  clusters,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Create handles POST /api/v0/clusters
func (h *ClusterHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)

	var req types.ClusterCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, ErrClusterCreateInvalidBody)
		return
	}

	if req.Name == "" || req.Spec == nil {
		writeAPIError(w, ErrClusterCreateMissingFields)
		return
	}

	if len(req.Name) > hyperfleetdb.MaxClusterNameLen {
		writeAPIError(w, ErrClusterCreateNameTooLong)
		return
	}

	if errs := h.validator.ValidateCreate(req.Spec, featuregate.Default); errs != nil {
		writeAPIError(w, ErrClusterValidation.WithErrors(errs))
		return
	}

	existing, err := h.db.ListClusters(ctx, accountID)
	if err != nil {
		h.logger.Error("failed to check cluster name uniqueness", "error", err, "account_id", accountID)
		writeAPIError(w, ErrClusterCreateNameCheck)
		return
	}
	for i := range existing.Items {
		if existing.Items[i].Name == req.Name {
			writeAPIError(w, ErrClusterCreateNameConflict.WithReason(req.Name))
			return
		}
	}

	if callerARN := middleware.GetCallerARN(ctx); callerARN != "" {
		req.Spec.CreatorARN = callerARN
	}

	clusterID := h.generateID()

	const maxHash4Retries = 5
	for attempt := 0; attempt < maxHash4Retries; attempt++ {
		h.logger.Info("creating cluster", "account_id", accountID, "cluster_name", req.Name, "cluster_id", clusterID)

		cr, err := hyperfleetdb.PlatformCreateToClusterCR(clusterID, accountID, &req)
		if err != nil {
			h.logger.Error("failed to convert cluster spec", "error", err, "account_id", accountID)
			writeAPIError(w, ErrClusterCreateInvalidSpec)
			return
		}

		if h.defaultClusterExpiration > 0 && cr.Spec.ExpirationTimestamp == nil {
			expiry := metav1.NewTime(time.Now().Add(h.defaultClusterExpiration))
			cr.Spec.ExpirationTimestamp = &expiry
		}

		if h.oidcIssuerBaseURL != "" {
			cr.Spec.HostedCluster.IssuerURL = h.oidcIssuerBaseURL + "/" + clusterID
		}

		if err := h.db.CreateCluster(ctx, accountID, cr); err != nil {
			if hyperfleetdb.IsAlreadyExists(err) && attempt < maxHash4Retries-1 {
				clusterID = h.generateID()
				continue
			}
			h.logger.Error("failed to create cluster", "error", err, "account_id", accountID)
			if hyperfleetdb.IsAlreadyExists(err) {
				writeAPIError(w, ErrClusterCreateIDExhausted)
				return
			}
			writeAPIError(w, ErrClusterCreateFailed)
			return
		}

		cluster := hyperfleetdb.ClusterCRToPlatform(cr)
		h.writeJSON(w, http.StatusCreated, cluster)
		return
	}
}

// Get handles GET /api/v0/clusters/{id}
func (h *ClusterHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	clusterID := vars["id"]

	h.logger.Info("getting cluster", "account_id", accountID, "cluster_id", clusterID)

	cr, err := h.db.GetCluster(ctx, accountID, clusterID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrClusterGetNotFound)
			return
		}
		h.logger.Error("failed to get cluster", "error", err, "account_id", accountID, "cluster_id", clusterID)
		writeAPIError(w, ErrClusterGetFailed)
		return
	}

	h.writeJSON(w, http.StatusOK, hyperfleetdb.ClusterCRToPlatform(cr))
}

// Update handles PUT /api/v0/clusters/{id}
func (h *ClusterHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	clusterID := vars["id"]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAPIError(w, ErrClusterUpdateInvalidBody)
		return
	}

	var req types.ClusterUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAPIError(w, ErrClusterUpdateInvalidBody)
		return
	}

	if req.Spec == nil {
		writeAPIError(w, ErrClusterUpdateMissingFields)
		return
	}

	h.logger.Info("updating cluster", "account_id", accountID, "cluster_id", clusterID)

	cr, err := h.db.GetCluster(ctx, accountID, clusterID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrClusterUpdateNotFound)
			return
		}
		h.logger.Error("failed to get cluster for update", "error", err, "account_id", accountID, "cluster_id", clusterID)
		writeAPIError(w, ErrClusterUpdateFailed)
		return
	}

	if errs := h.validator.ValidateUpdate(req.Spec, &cr.Spec, featuregate.Default); errs != nil {
		writeAPIError(w, ErrClusterValidation.WithErrors(errs))
		return
	}

	// Extract raw "spec" JSON from the request body so the merge only
	// overwrites fields the caller actually sent, preserving service-set
	// fields that lack omitempty (e.g. hostedCluster, nodePool).
	var envelope struct {
		Spec json.RawMessage `json:"spec"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		writeAPIError(w, ErrClusterUpdateInvalidBody)
		return
	}

	if err := hyperfleetdb.MergeSpecJSON(&cr.Spec, envelope.Spec); err != nil {
		h.logger.Error("failed to merge cluster spec", "error", err)
		writeAPIError(w, ErrClusterUpdateInvalidSpec)
		return
	}

	if err := h.db.UpdateCluster(ctx, cr); err != nil {
		h.logger.Error("failed to update cluster", "error", err, "account_id", accountID, "cluster_id", clusterID)
		writeAPIError(w, ErrClusterUpdateFailed)
		return
	}

	h.writeJSON(w, http.StatusOK, hyperfleetdb.ClusterCRToPlatform(cr))
}

// Delete handles DELETE /api/v0/clusters/{id}
func (h *ClusterHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	clusterID := vars["id"]

	h.logger.Info("deleting cluster", "account_id", accountID, "cluster_id", clusterID)

	err := h.db.DeleteCluster(ctx, accountID, clusterID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrClusterDeleteNotFound)
			return
		}
		h.logger.Error("failed to delete cluster", "error", err, "account_id", accountID, "cluster_id", clusterID)
		writeAPIError(w, ErrClusterDeleteFailed)
		return
	}

	response := map[string]any{
		"message":    "Cluster deletion initiated",
		"cluster_id": clusterID,
	}

	h.writeJSON(w, http.StatusAccepted, response)
}

// GetStatus handles GET /api/v0/clusters/{id}/statuses
func (h *ClusterHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	clusterID := vars["id"]

	h.logger.Info("getting cluster status", "account_id", accountID, "cluster_id", clusterID)

	cr, err := h.db.GetCluster(ctx, accountID, clusterID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrClusterStatusNotFound)
			return
		}
		h.logger.Error("failed to get cluster status", "error", err, "account_id", accountID, "cluster_id", clusterID)
		writeAPIError(w, ErrClusterStatusFailed)
		return
	}

	h.writeJSON(w, http.StatusOK, hyperfleetdb.ClusterStatusFromCR(cr))
}

func (h *ClusterHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
