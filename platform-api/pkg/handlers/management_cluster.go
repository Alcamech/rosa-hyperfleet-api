package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hyperfleetv1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/clients/hyperfleetdb"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/middleware"
)

// ManagementClusterHandler handles management cluster endpoints.
type ManagementClusterHandler struct {
	db     *hyperfleetdb.Client
	logger *slog.Logger
}

// NewManagementClusterHandler creates a new ManagementClusterHandler.
func NewManagementClusterHandler(db *hyperfleetdb.Client, logger *slog.Logger) *ManagementClusterHandler {
	return &ManagementClusterHandler{
		db:     db,
		logger: logger,
	}
}

// ManagementClusterCreateRequest is the request body for creating an MC registration.
type ManagementClusterCreateRequest struct {
	ID        string `json:"id"`
	Region    string `json:"region"`
	AccountID string `json:"accountId"`
}

// ManagementClusterResponse is the JSON response for a management cluster.
type ManagementClusterResponse struct {
	ID        string `json:"id"`
	Region    string `json:"region"`
	AccountID string `json:"accountId"`
}

// Create handles POST /api/v0/management_clusters
func (h *ManagementClusterHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)

	h.logger.Info("creating management cluster", "account_id", accountID)

	var req ManagementClusterCreateRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, ErrMCCreateInvalidBody)
			return
		}
	}

	if req.ID == "" {
		writeAPIError(w, ErrMCCreateMissingID)
		return
	}
	if req.Region == "" {
		writeAPIError(w, ErrMCCreateMissingReg)
		return
	}
	if req.AccountID == "" {
		writeAPIError(w, ErrMCCreateMissingAcct)
		return
	}

	mc := &hyperfleetv1alpha1.ManagementCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.ID,
		},
		Spec: hyperfleetv1alpha1.ManagementClusterSpec{
			Region:    req.Region,
			AccountID: req.AccountID,
		},
	}

	if err := h.db.CreateManagementCluster(ctx, mc); err != nil {
		if hyperfleetdb.IsAlreadyExists(err) {
			writeAPIError(w, ErrMCCreateExists.WithReason(req.ID))
			return
		}
		h.logger.Error("failed to create management cluster", "error", err)
		writeAPIError(w, ErrMCCreateFailed)
		return
	}

	h.logger.Info("management cluster created", "id", mc.Name, "account_id", accountID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(mcToResponse(mc))
}

// List handles GET /api/v0/management_clusters
func (h *ManagementClusterHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)

	h.logger.Debug("listing management clusters", "account_id", accountID)

	list, err := h.db.ListManagementClusters(ctx)
	if err != nil {
		h.logger.Error("failed to list management clusters", "error", err)
		writeAPIError(w, ErrMCListFailed)
		return
	}

	clusters := make([]ManagementClusterResponse, 0, len(list.Items))
	for i := range list.Items {
		clusters = append(clusters, mcToResponse(&list.Items[i]))
	}

	h.logger.Debug("management clusters listed", "total", len(clusters), "account_id", accountID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"kind":  "ManagementClusterList",
		"items": clusters,
		"total": len(clusters),
	})
}

// Get handles GET /api/v0/management_clusters/{id}
func (h *ManagementClusterHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	id := vars["id"]

	h.logger.Debug("getting management cluster", "id", id, "account_id", accountID)

	mc, err := h.db.GetManagementCluster(ctx, id)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrMCGetNotFound)
			return
		}
		h.logger.Error("failed to get management cluster", "error", err, "id", id)
		writeAPIError(w, ErrMCGetFailed)
		return
	}

	h.logger.Debug("management cluster retrieved", "id", mc.Name, "account_id", accountID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mcToResponse(mc))
}

func mcToResponse(mc *hyperfleetv1alpha1.ManagementCluster) ManagementClusterResponse {
	return ManagementClusterResponse{
		ID:        mc.Name,
		Region:    mc.Spec.Region,
		AccountID: mc.Spec.AccountID,
	}
}
