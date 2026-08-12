package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/internal/codegen/featuregate"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/clients/hyperfleetdb"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/middleware"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/types"
	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/validation"

	"github.com/google/uuid"
)

type NodePoolHandler struct {
	db        *hyperfleetdb.Client
	validator *validation.FieldValidator
	logger    *slog.Logger
}

func NewNodePoolHandler(db *hyperfleetdb.Client, logger *slog.Logger) *NodePoolHandler {
	return &NodePoolHandler{
		db:        db,
		validator: validation.NewFieldValidator(),
		logger:    logger,
	}
}

func (h *NodePoolHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	clusterID := r.URL.Query().Get("clusterId")

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

	h.logger.Info("listing nodepools", "account_id", accountID, "limit", limit, "offset", offset, "cluster_id", clusterID)

	list, err := h.db.ListNodePools(ctx, accountID, clusterID)
	if err != nil {
		h.logger.Error("failed to list nodepools", "error", err, "account_id", accountID)
		writeAPIError(w, ErrNodePoolList)
		return
	}

	nodepools := make([]*types.NodePool, 0, len(list.Items))
	for i := range list.Items {
		nodepools = append(nodepools, hyperfleetdb.NodePoolCRToPlatform(&list.Items[i]))
	}

	total := len(nodepools)

	if offset >= len(nodepools) {
		nodepools = nil
	} else {
		end := min(offset+limit, len(nodepools))
		nodepools = nodepools[offset:end]
	}

	response := map[string]any{
		"items":  nodepools,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	h.writeJSON(w, http.StatusOK, response)
}

func (h *NodePoolHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)

	var req types.NodePoolCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, ErrNodePoolCreateInvalidBody)
		return
	}

	if req.Name == "" || req.ClusterID == "" || req.Spec == nil {
		writeAPIError(w, ErrNodePoolCreateMissingFields)
		return
	}

	if errs := h.validator.ValidateCreate(req.Spec, featuregate.Default); errs != nil {
		def := ErrNodePoolValidation
		def.Errors = errs
		writeAPIError(w, def)
		return
	}

	if _, err := h.db.GetCluster(ctx, accountID, req.ClusterID); err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrNodePoolCreateClusterNotFound)
			return
		}
		h.logger.Error("failed to verify cluster exists", "error", err, "account_id", accountID, "cluster_id", req.ClusterID)
		writeAPIError(w, ErrNodePoolCreateClusterCheck)
		return
	}

	h.logger.Info("creating nodepool", "account_id", accountID, "cluster_id", req.ClusterID, "nodepool_name", req.Name)

	internalPoolID := uuid.New().String()
	cr, err := hyperfleetdb.PlatformCreateToNodePoolCR(accountID, internalPoolID, &req)
	if err != nil {
		h.logger.Error("failed to convert nodepool spec", "error", err, "account_id", accountID)
		writeAPIError(w, ErrNodePoolCreateInvalidSpec)
		return
	}

	if err := h.db.CreateNodePool(ctx, accountID, cr); err != nil {
		h.logger.Error("failed to create nodepool", "error", err, "account_id", accountID)
		if hyperfleetdb.IsAlreadyExists(err) {
			writeAPIError(w, ErrNodePoolCreateNameConflict)
			return
		}
		writeAPIError(w, ErrNodePoolCreateFailed)
		return
	}

	h.writeJSON(w, http.StatusCreated, hyperfleetdb.NodePoolCRToPlatform(cr))
}

func (h *NodePoolHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	nodepoolID := vars["id"]

	h.logger.Info("getting nodepool", "account_id", accountID, "nodepool_id", nodepoolID)

	cr, err := h.db.GetNodePool(ctx, accountID, nodepoolID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrNodePoolGetNotFound)
			return
		}
		h.logger.Error("failed to get nodepool", "error", err, "account_id", accountID, "nodepool_id", nodepoolID)
		writeAPIError(w, ErrNodePoolGetFailed)
		return
	}

	h.writeJSON(w, http.StatusOK, hyperfleetdb.NodePoolCRToPlatform(cr))
}

func (h *NodePoolHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	nodepoolID := vars["id"]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAPIError(w, ErrNodePoolUpdateInvalidBody)
		return
	}

	var req types.NodePoolUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAPIError(w, ErrNodePoolUpdateInvalidBody)
		return
	}

	if req.Spec == nil {
		writeAPIError(w, ErrNodePoolUpdateMissingFields)
		return
	}

	h.logger.Info("updating nodepool", "account_id", accountID, "nodepool_id", nodepoolID)

	cr, err := h.db.GetNodePool(ctx, accountID, nodepoolID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrNodePoolUpdateNotFound)
			return
		}
		h.logger.Error("failed to get nodepool for update", "error", err, "account_id", accountID, "nodepool_id", nodepoolID)
		writeAPIError(w, ErrNodePoolUpdateFailed)
		return
	}

	if errs := h.validator.ValidateUpdate(req.Spec, &cr.Spec, featuregate.Default); errs != nil {
		def := ErrNodePoolValidation
		def.Errors = errs
		writeAPIError(w, def)
		return
	}

	var envelope struct {
		Spec json.RawMessage `json:"spec"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		writeAPIError(w, ErrNodePoolUpdateInvalidBody)
		return
	}

	if err := hyperfleetdb.MergeSpecJSON(&cr.Spec, envelope.Spec); err != nil {
		h.logger.Error("failed to merge nodepool spec", "error", err)
		writeAPIError(w, ErrNodePoolUpdateInvalidSpec)
		return
	}

	if err := h.db.UpdateNodePool(ctx, cr); err != nil {
		h.logger.Error("failed to update nodepool", "error", err, "account_id", accountID, "nodepool_id", nodepoolID)
		writeAPIError(w, ErrNodePoolUpdateFailed)
		return
	}

	h.writeJSON(w, http.StatusOK, hyperfleetdb.NodePoolCRToPlatform(cr))
}

func (h *NodePoolHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	nodepoolID := vars["id"]

	h.logger.Info("deleting nodepool", "account_id", accountID, "nodepool_id", nodepoolID)

	err := h.db.DeleteNodePool(ctx, accountID, nodepoolID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrNodePoolDeleteNotFound)
			return
		}
		h.logger.Error("failed to delete nodepool", "error", err, "account_id", accountID, "nodepool_id", nodepoolID)
		writeAPIError(w, ErrNodePoolDeleteFailed)
		return
	}

	response := map[string]any{
		"message":     "NodePool deletion initiated",
		"nodepool_id": nodepoolID,
	}

	h.writeJSON(w, http.StatusAccepted, response)
}

func (h *NodePoolHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := middleware.GetAccountID(ctx)
	vars := mux.Vars(r)
	nodepoolID := vars["id"]

	h.logger.Info("getting nodepool status", "account_id", accountID, "nodepool_id", nodepoolID)

	cr, err := h.db.GetNodePool(ctx, accountID, nodepoolID)
	if err != nil {
		if hyperfleetdb.IsNotFound(err) {
			writeAPIError(w, ErrNodePoolStatusNotFound)
			return
		}
		h.logger.Error("failed to get nodepool status", "error", err, "account_id", accountID, "nodepool_id", nodepoolID)
		writeAPIError(w, ErrNodePoolStatusFailed)
		return
	}

	h.writeJSON(w, http.StatusOK, hyperfleetdb.NodePoolStatusFromCR(cr))
}

func (h *NodePoolHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
