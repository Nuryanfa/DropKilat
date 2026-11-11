package db

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubTx struct {
	pgx.Tx
	execErr, commitErr    error
	committed, rolledBack bool
	values                []any
}

func (s *stubTx) Exec(_ context.Context, _ string, values ...any) (pgconn.CommandTag, error) {
	s.values = values
	return pgconn.CommandTag{}, s.execErr
}
func (s *stubTx) Commit(context.Context) error { s.committed = true; return s.commitErr }
func (s *stubTx) Rollback(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.rolledBack = true
	if s.committed && s.commitErr == nil {
		return pgx.ErrTxClosed
	}
	return nil
}

type stubBegin struct {
	tx      *stubTx
	options pgx.TxOptions
	err     error
}

func (s *stubBegin) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	s.options = options
	return s.tx, s.err
}

func TestTransactionLifecycle(t *testing.T) {
	failure := errors.New("failure")
	for _, scenario := range []string{"success", "begin", "context", "callback", "commit", "cancelled", "panic"} {
		t.Run(scenario, func(t *testing.T) {
			tx := &stubTx{}
			pool := &stubBegin{tx: tx}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "begin":
				pool.err = failure
			case "context":
				tx.execErr = failure
			case "commit":
				tx.commitErr = failure
			}
			called := false
			callback := func(pgx.Tx) error {
				called = true
				switch scenario {
				case "callback":
					return failure
				case "cancelled":
					cancel()
					return failure
				case "panic":
					panic(failure)
				}
				return nil
			}
			if scenario == "panic" {
				defer func() {
					if recover() != failure || !tx.rolledBack || tx.committed {
						t.Fatal("panic must be preserved after rollback")
					}
				}()
			}
			err := withTx(ctx, pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, "public", "", "", callback)
			if scenario == "panic" {
				t.Fatal("panic swallowed")
			}
			if scenario == "success" && err != nil {
				t.Fatal(err)
			}
			if scenario != "success" && !errors.Is(err, failure) {
				t.Fatalf("lost error: %v", err)
			}
			if scenario == "begin" {
				if tx.rolledBack || called {
					t.Fatal("used transaction after begin error")
				}
				return
			}
			if !tx.rolledBack {
				t.Fatal("transaction was not cleaned up")
			}
			if tx.committed != (scenario == "success" || scenario == "commit") {
				t.Fatal("unexpected commit")
			}
			if called != (scenario != "context") {
				t.Fatal("unexpected callback")
			}
			if pool.options.AccessMode != pgx.ReadOnly || !reflect.DeepEqual(tx.values, []any{"public", "", ""}) {
				t.Fatal("wrong context or access mode")
			}
		})
	}
}

func TestEmbeddedMigrationVersion(t *testing.T) {
	version, err := LatestVersion()
	if err != nil || version != 2 {
		t.Fatalf("version = %d, error = %v", version, err)
	}
	if err := Migrate(t.Context(), "invalid", "invalid"); err == nil {
		t.Fatal("invalid direction accepted")
	}
}
