package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockDBTX is a minimal DBTX implementation suitable for unit tests.
// It records the last SQL string passed to each method but does nothing else.
type mockDBTX struct {
	lastExecSQL     string
	lastQuerySQL    string
	lastQueryRowSQL string
}

func (m *mockDBTX) Exec(_ context.Context, sql string, _ ...interface{}) (pgconn.CommandTag, error) {
	m.lastExecSQL = sql
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTX) Query(_ context.Context, sql string, _ ...interface{}) (pgx.Rows, error) {
	m.lastQuerySQL = sql
	return nil, nil
}

func (m *mockDBTX) QueryRow(_ context.Context, sql string, _ ...interface{}) pgx.Row {
	m.lastQueryRowSQL = sql
	return nil
}

// ─────────────────────────────────────────────────────────────
// New
// ─────────────────────────────────────────────────────────────

func TestNew_ReturnsNonNilQueries(t *testing.T) {
	mock := &mockDBTX{}
	q := New(mock)
	if q == nil {
		t.Fatal("New() returned nil *Queries")
	}
}

func TestNew_StoresDbtx(t *testing.T) {
	mock := &mockDBTX{}
	q := New(mock)
	// Verify the stored DBTX is the same instance by calling Exec through it
	// and checking the side effect on the mock.
	_, _ = q.db.Exec(context.Background(), "SELECT 1")
	if mock.lastExecSQL != "SELECT 1" {
		t.Errorf("New() stored wrong DBTX: Exec recorded %q, want %q", mock.lastExecSQL, "SELECT 1")
	}
}

func TestNew_DifferentCallsReturnIndependentInstances(t *testing.T) {
	mock1 := &mockDBTX{}
	mock2 := &mockDBTX{}
	q1 := New(mock1)
	q2 := New(mock2)
	if q1 == q2 {
		t.Error("New() returned the same *Queries instance for different DBTX inputs")
	}
}

// ─────────────────────────────────────────────────────────────
// WithTx
// ─────────────────────────────────────────────────────────────

func TestWithTx_ReturnsNewQueriesInstance(t *testing.T) {
	mock := &mockDBTX{}
	q := New(mock)

	// mockTx satisfies pgx.Tx; we only need the fact that WithTx wraps it.
	// Use a nil pgx.Tx: the returned *Queries should be non-nil even if the
	// underlying tx is nil (construction must not panic).
	var tx pgx.Tx // nil interface value satisfies pgx.Tx
	newQ := q.WithTx(tx)
	if newQ == nil {
		t.Fatal("WithTx() returned nil *Queries")
	}
	if newQ == q {
		t.Error("WithTx() returned the same *Queries pointer, expected a new instance")
	}
}

// ─────────────────────────────────────────────────────────────
// DBTX interface satisfaction
// ─────────────────────────────────────────────────────────────

// TestMockDBTXSatisfiesDTBX verifies our mock satisfies DBTX so tests are trustworthy.
func TestMockDBTXSatisfiesDTBX(t *testing.T) {
	var _ DBTX = (*mockDBTX)(nil)
}
