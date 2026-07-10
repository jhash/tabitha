package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jhash/tabitha/internal/db"
)

func TestHealthzReturns200WhenDatabaseReachable(t *testing.T) {
	q := setupTestQueries(t)

	rec := httptest.NewRecorder()
	HealthzHandler(q)(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// failingDB implements db.DBTX with every method failing, so
// HealthzHandler can be tested against an unreachable database without a
// real outage.
type failingDB struct{}

func (failingDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("simulated db failure")
}

func (failingDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("simulated db failure")
}

func (failingDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return failingRow{}
}

type failingRow struct{}

func (failingRow) Scan(...any) error { return errors.New("simulated db failure") }

func TestHealthzReturns503WhenDatabaseUnreachable(t *testing.T) {
	q := db.New(failingDB{})

	rec := httptest.NewRecorder()
	HealthzHandler(q)(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
