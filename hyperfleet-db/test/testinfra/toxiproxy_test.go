package testinfra_test

import (
	"context"
	"testing"

	"github.com/openshift-online/rosa-hyperfleet-api/hyperfleet-db/test/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxiedDBSetup(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres + toxiproxy")
	}

	pdb := testinfra.StartPostgresWithProxy()
	defer pdb.Stop()

	ctx := context.Background()

	// Direct connection works
	direct, err := pdb.DirectConn(ctx)
	require.NoError(t, err)
	defer func() { _ = direct.Close(ctx) }()

	var one int
	err = direct.QueryRow(ctx, "SELECT 1").Scan(&one)
	require.NoError(t, err)
	assert.Equal(t, 1, one)

	// Proxied connection works
	proxied, err := pdb.ProxiedConn(ctx)
	require.NoError(t, err)
	defer func() { _ = proxied.Close(ctx) }()

	err = proxied.QueryRow(ctx, "SELECT 1").Scan(&one)
	require.NoError(t, err)
	assert.Equal(t, 1, one)

	// Disable proxy → proxied connection should fail
	_ = pdb.Proxy.Disable()
	proxied2, err := pdb.ProxiedConn(ctx)
	if err == nil {
		var result int
		err = proxied2.QueryRow(ctx, "SELECT 1").Scan(&result)
		assert.Error(t, err, "query through disabled proxy must fail")
		_ = proxied2.Close(ctx)
	}

	// Re-enable → works again
	_ = pdb.Proxy.Enable()
	proxied3, err := pdb.ProxiedConn(ctx)
	require.NoError(t, err)
	defer func() { _ = proxied3.Close(ctx) }()

	err = proxied3.QueryRow(ctx, "SELECT 1").Scan(&one)
	require.NoError(t, err)
}
