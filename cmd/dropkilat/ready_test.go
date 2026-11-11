package main

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadinessResponse(t *testing.T) {
	for _, healthy := range []bool{true, false} {
		result := httptest.NewRecorder()
		handler := readyHandler(func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 2*time.Second {
				t.Error("readiness has no bounded deadline")
			}
			if !healthy {
				return errors.New("secret-database-details")
			}
			return nil
		})
		handler.ServeHTTP(result, httptest.NewRequest("GET", "/readyz", nil))
		status, body := 200, `{"status":"ok"}`
		if !healthy {
			status, body = 503, `{"status":"unavailable"}`
		}
		if result.Code != status || result.Body.String() != body || result.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected readiness response: %v", result)
		}
	}
}

func TestMigrateCommandArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"wrong"}, {"migrate"}, {"migrate", "reset"}, {"migrate", "up", "extra"}} {
		if err := migrateCommand(t.Context(), args, func(string) string { return "" }, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	err := migrateCommand(t.Context(), []string{"migrate", "up"}, func(string) string { return "" }, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "DK_MIGRATE_DATABASE_URL") {
		t.Fatalf("missing owner URL: %v", err)
	}
}
