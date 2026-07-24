package race_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openshift-online/rosa-hyperfleet-api/hyperfleet-db/internal/doorbell"
	"github.com/openshift-online/rosa-hyperfleet-api/hyperfleet-db/internal/model"
	"github.com/openshift-online/rosa-hyperfleet-api/hyperfleet-db/internal/writer"
	"github.com/openshift-online/rosa-hyperfleet-api/hyperfleet-db/test/testinfra"
)

var sharedDB *testinfra.TestDB

func TestMain(m *testing.M) {
	sharedDB = testinfra.StartPostgresForTestMain()
	code := m.Run()
	sharedDB.Stop()
	os.Exit(code)
}

func freshConn(t *testing.T) *pgx.Conn {
	t.Helper()
	return sharedDB.Connect(t)
}

func truncateAll(t *testing.T) {
	t.Helper()
	conn := freshConn(t)
	sharedDB.TruncateAll(t, conn)
	_ = conn.Close(context.Background())
}

func makeWriteReq(gvk, ns, name string) model.WriteRequest { //nolint:unparam
	return model.WriteRequest{
		GVK:       gvk,
		Namespace: ns,
		Name:      name,
		Spec:      json.RawMessage(`{"replicas":1}`),
		Status:    json.RawMessage(`{}`),
		Metadata:  json.RawMessage(`{}`),
	}
}

func newWriter(t *testing.T, hooks writer.TxHooks) *writer.Writer {
	t.Helper()
	dbConn := freshConn(t)
	db := doorbell.NewDebouncer(dbConn, 50*time.Millisecond)
	t.Cleanup(func() { db.Close() })
	return writer.New(freshConn(t), hooks).WithDoorbell(db)
}

// blockingHook implements TxHooks to pause at BeforeCommit.
type blockingHook struct {
	ready   chan struct{} // closed when the hook is entered
	proceed chan struct{} // closed to let the hook return
}

func newBlockingHook() *blockingHook {
	return &blockingHook{
		ready:   make(chan struct{}),
		proceed: make(chan struct{}),
	}
}

func (h *blockingHook) AfterSuppressionCheck(_ context.Context, _ pgx.Tx, _ bool) error { return nil }
func (h *blockingHook) AfterTxidAcquire(_ context.Context, _ pgx.Tx, _ uint64) error    { return nil }

func (h *blockingHook) BeforeCommit(ctx context.Context, _ pgx.Tx) error {
	close(h.ready)
	select {
	case <-h.proceed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
