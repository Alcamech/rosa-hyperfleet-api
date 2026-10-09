package handlers

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"

	v1 "github.com/openshift-online/rosa-seaplane-api/api/v1alpha1"
	"github.com/openshift-online/rosa-seaplane-api/seaplane-api/pkg/authz"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Creation is authorized before FleetDB assigns a UID.
func candidateResource(resource authz.Resource, name string) authz.Resource {
	resource.ID = name
	return resource
}

func (h *NodePoolHandler) nodePoolResource(ctx context.Context, np *v1.NodePool) (authz.Resource, error) {
	owner := metav1.GetControllerOf(np)
	if owner == nil || owner.Kind != "Cluster" || owner.UID == "" || np.Labels[clusterUIDLabel] != string(owner.UID) {
		return authz.Resource{}, &authz.Failure{Stage: authz.StageEntityValidation, Err: fmt.Errorf("invalid node pool parent binding")}
	}
	// Resolve the stored controller reference within the caller's account and pin its incarnation.
	parent, err := h.db.GetCluster(ctx, owner.Name)
	if err != nil {
		return authz.Resource{}, &authz.Failure{Stage: authz.StageResourceLoading, Err: err}
	}
	if parent.UID != owner.UID {
		return authz.Resource{}, &authz.Failure{Stage: authz.StageEntityValidation, Err: fmt.Errorf("node pool parent UID does not match")}
	}
	return authz.Resource{Kind: authz.NodePool, ID: string(np.UID), AccountID: np.Labels["seaplane.io/account-id"], Labels: maps.Clone(np.Labels),
		ParentCluster: &authz.ParentCluster{ID: string(parent.UID), AccountID: parent.Labels["seaplane.io/account-id"], Labels: maps.Clone(parent.Labels)}}, nil
}
func (h *NodePoolHandler) authorizeNodePool(w http.ResponseWriter, r *http.Request, operation authz.Action, np *v1.NodePool, additional ...string) bool {
	resource, err := h.nodePoolResource(r.Context(), np)
	if err != nil {
		attempt, _ := authz.DefaultMetrics.Start(operation)
		writeResourceAuthzError(w, r, attempt, operation, err, h.logger)
		return false
	}
	if operation == authz.CreateNodePool {
		resource = candidateResource(resource, np.Name)
	}
	return authorizeResource(w, r, h.authorizer, h.logger, operation, resource, additional...)
}

func (h *OidcConfigHandler) oidcConfigResource(oc *v1.OidcConfig) authz.Resource {
	account := strings.TrimPrefix(oc.Namespace, "account-")
	if account == oc.Namespace {
		account = ""
	}
	return authz.Resource{Kind: authz.OIDCConfig, ID: string(oc.UID), AccountID: account, Labels: maps.Clone(oc.Labels)}
}
func (h *ManagementClusterHandler) managementClusterResource(mc *v1.ManagementCluster) authz.Resource {
	id := mc.Name
	if mc.Namespace != "" {
		id = ""
	}
	return authz.Resource{Kind: authz.ManagementCluster, ID: id, AccountID: mc.Spec.AccountID, RegistrationRegion: mc.Spec.Region, Labels: maps.Clone(mc.Labels)}
}
