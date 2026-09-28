package hyperfleetdb

import (
	"context"
	"log/slog"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hyperfleetv1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = hyperfleetv1alpha1.AddToScheme(s)
	return s
}

func TestClient_CreateCluster_SetsAccountLabel(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := NewClientFrom(fc, logger)

	cluster := &hyperfleetv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "cluster-uuid-1",
		},
	}

	if err := c.CreateCluster(context.Background(), "acct-123", cluster); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	if got := cluster.Labels[accountIDLabel]; got != "acct-123" {
		t.Errorf("account-id label = %q, want %q", got, "acct-123")
	}
}

func TestClient_CreateNodePool_SetsNamespaceAndLabel(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		&hyperfleetv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "parent", Namespace: "cluster-uuid-1", Labels: map[string]string{accountIDLabel: "acct-123"},
		}},
	).Build()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := NewClientFrom(fc, logger)

	np := &hyperfleetv1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-nodepool",
			Namespace: "cluster-uuid-1",
		},
		Spec: hyperfleetv1alpha1.NodePoolSpec{},
	}

	if err := c.CreateNodePool(context.Background(), "acct-123", np); err != nil {
		t.Fatalf("CreateNodePool: %v", err)
	}

	if got := np.Labels[accountIDLabel]; got != "acct-123" {
		t.Errorf("account-id label = %q, want %q", got, "acct-123")
	}
	if np.Spec.AccountID != "acct-123" {
		t.Errorf("spec.accountID = %q, want %q", np.Spec.AccountID, "acct-123")
	}
}

func TestClient_CreateNodePool_RequiresOwnedParent(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		&hyperfleetv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "foreign", Namespace: "cluster-uuid-2", Labels: map[string]string{accountIDLabel: "acct-2"},
		}},
	).Build()
	c := NewClientFrom(fc, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	np := &hyperfleetv1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{Name: "workers", Namespace: "cluster-uuid-2"},
	}
	if err := c.CreateNodePool(context.Background(), "acct-1", np); !IsNotFound(err) {
		t.Fatalf("CreateNodePool into foreign parent: want NotFound, got %v", err)
	}
}

func TestClient_UpdateCluster_RepinsAccount(t *testing.T) {
	cr := &hyperfleetv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "owned", Namespace: "cluster-uuid-1",
			Labels: map[string]string{accountIDLabel: "acct-1"},
		},
		Spec: hyperfleetv1alpha1.ClusterSpec{AccountID: "acct-1", DisplayName: "before"},
	}
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(cr).Build()
	c := NewClientFrom(fc, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cr.Labels[accountIDLabel] = "forged"
	cr.Spec.AccountID = "forged"
	cr.Spec.DisplayName = "after"
	if err := c.UpdateCluster(context.Background(), "acct-1", cr); err != nil {
		t.Fatalf("UpdateCluster: %v", err)
	}
	if cr.Labels[accountIDLabel] != "acct-1" || cr.Spec.AccountID != "acct-1" {
		t.Errorf("ownership = (%q, %q), want acct-1", cr.Labels[accountIDLabel], cr.Spec.AccountID)
	}
	if cr.Spec.DisplayName != "after" {
		t.Errorf("displayName = %q, want after", cr.Spec.DisplayName)
	}
}

func TestClient_UpdateNodePool_RepinsAccount(t *testing.T) {
	np := &hyperfleetv1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workers", Namespace: "cluster-uuid-1",
			Labels: map[string]string{accountIDLabel: "acct-1"},
		},
		Spec: hyperfleetv1alpha1.NodePoolSpec{AccountID: "acct-1", DisplayName: "before"},
	}
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(np).Build()
	c := NewClientFrom(fc, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	np.Labels[accountIDLabel] = "forged"
	np.Spec.AccountID = "forged"
	np.Spec.DisplayName = "after"
	if err := c.UpdateNodePool(context.Background(), "acct-1", np); err != nil {
		t.Fatalf("UpdateNodePool: %v", err)
	}
	if np.Labels[accountIDLabel] != "acct-1" || np.Spec.AccountID != "acct-1" {
		t.Errorf("ownership = (%q, %q), want acct-1", np.Labels[accountIDLabel], np.Spec.AccountID)
	}
	if np.Spec.DisplayName != "after" {
		t.Errorf("displayName = %q, want after", np.Spec.DisplayName)
	}
}

func TestClient_ListClusters_FiltersByAccount(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		&hyperfleetv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-cluster", Namespace: "cluster-uuid-1",
				Labels: map[string]string{accountIDLabel: "acct-1"},
			},
		},
		&hyperfleetv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-cluster", Namespace: "cluster-uuid-2",
				Labels: map[string]string{accountIDLabel: "acct-2"},
			},
		},
	).Build()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := NewClientFrom(fc, logger)

	list, err := c.ListClusters(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 cluster for acct-1, got %d", len(list.Items))
	}
	if list.Items[0].Name != "my-cluster" {
		t.Errorf("got cluster %q, want my-cluster", list.Items[0].Name)
	}
}

func TestClient_GetNodePool_ScopedToCluster(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		&hyperfleetv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "one", Namespace: "cluster-uuid-1", Labels: map[string]string{accountIDLabel: "acct-1"},
		}},
		&hyperfleetv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "two", Namespace: "cluster-uuid-2", Labels: map[string]string{accountIDLabel: "acct-1"},
		}},
		&hyperfleetv1alpha1.NodePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "workers", Namespace: "cluster-uuid-1",
				Labels: map[string]string{accountIDLabel: "acct-1"},
			},
		},
		&hyperfleetv1alpha1.NodePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "workers", Namespace: "cluster-uuid-2",
				Labels: map[string]string{accountIDLabel: "acct-1"},
			},
		},
	).Build()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := NewClientFrom(fc, logger)

	np, err := c.GetNodePool(context.Background(), "acct-1", "uuid-2", "workers")
	if err != nil {
		t.Fatalf("GetNodePool: %v", err)
	}
	if np.Namespace != "cluster-uuid-2" {
		t.Errorf("namespace = %q, want cluster-uuid-2", np.Namespace)
	}
}

func TestClient_GetNodePool_FallbackWithoutClusterID(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		&hyperfleetv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "one", Namespace: "cluster-uuid-1", Labels: map[string]string{accountIDLabel: "acct-1"},
		}},
		&hyperfleetv1alpha1.NodePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "workers", Namespace: "cluster-uuid-1",
				Labels: map[string]string{accountIDLabel: "acct-1"},
			},
		},
	).Build()
	c := NewClientFrom(fc, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	np, err := c.GetNodePool(context.Background(), "acct-1", "", "workers")
	if err != nil {
		t.Fatalf("GetNodePool empty clusterID: unexpected err = %v", err)
	}
	if np.Name != "workers" {
		t.Errorf("got nodepool %q, want workers", np.Name)
	}
}

func TestClient_GetNodePool_RequiresOwnedParent(t *testing.T) {
	ctx := context.Background()
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		&hyperfleetv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "own", Namespace: "cluster-uuid-1", Labels: map[string]string{accountIDLabel: "acct-1"},
		}},
		&hyperfleetv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "foreign", Namespace: "cluster-uuid-2", Labels: map[string]string{accountIDLabel: "acct-2"},
		}},
		&hyperfleetv1alpha1.NodePool{ObjectMeta: metav1.ObjectMeta{
			Name: "workers", Namespace: "cluster-uuid-1", Labels: map[string]string{accountIDLabel: "acct-1"},
		}},
		&hyperfleetv1alpha1.NodePool{ObjectMeta: metav1.ObjectMeta{
			Name: "foreign-workers", Namespace: "cluster-uuid-2", Labels: map[string]string{accountIDLabel: "acct-1"},
		}},
	).Build()
	c := NewClientFrom(fc, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	for _, clusterID := range []string{"uuid-2", ""} {
		if _, err := c.GetNodePool(ctx, "acct-1", clusterID, "foreign-workers"); !IsNotFound(err) {
			t.Errorf("GetNodePool(clusterID=%q) should hide foreign parent, got %v", clusterID, err)
		}
		list, err := c.ListNodePools(ctx, "acct-1", clusterID)
		if err != nil {
			t.Fatalf("ListNodePools(clusterID=%q): %v", clusterID, err)
		}
		if len(list.Items) != 0 && clusterID != "" {
			t.Errorf("scoped list revealed %d nodepools in foreign parent", len(list.Items))
		}
		if clusterID == "" && (len(list.Items) != 1 || list.Items[0].Namespace != "cluster-uuid-1") {
			t.Errorf("account list should contain only the owned nodepool, got %v", list.Items)
		}
	}

	if _, err := c.GetNodePool(ctx, "acct-1", "uuid-1", "workers"); err != nil {
		t.Errorf("owned nodepool should remain accessible: %v", err)
	}
}

func TestClient_GetCluster_ScopedToAccount(t *testing.T) {
	fc := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		&hyperfleetv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-cluster", Namespace: "cluster-uuid-1",
				Labels: map[string]string{accountIDLabel: "acct-1"},
			},
		},
	).Build()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := NewClientFrom(fc, logger)

	// Correct account can access
	cr, err := c.GetCluster(context.Background(), "acct-1", "uuid-1")
	if err != nil {
		t.Fatalf("GetCluster with correct account: %v", err)
	}
	if cr.Name != "my-cluster" {
		t.Errorf("got cluster %q, want my-cluster", cr.Name)
	}

	// Wrong account gets not-found
	_, err = c.GetCluster(context.Background(), "acct-2", "uuid-1")
	if !IsNotFound(err) {
		t.Errorf("GetCluster with wrong account: expected NotFound, got %v", err)
	}
}
