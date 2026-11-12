package httpx

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Nuryanfa/DropKilat/internal/platform/clock/clocktest"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProblems(t *testing.T) {
	cases := map[string]int{"drop-not-live": 409, "sold-out": 200, "already-claimed": 409, "not-verified": 401, "rate-limited": 429, "hold-expired": 409, "invalid-transition": 409, "seller-not-verified": 403, "new-seller-limit": 422, "validation": 422, "buyer-blocked": 403, "step-up-required": 401, "otomatis-not-eligible": 403, "invoice-overdue": 403}
	for typ, status := range cases {
		t.Run(typ, func(t *testing.T) {
			e := errors.New(typ)
			Register(e, typ, status)
			w := httptest.NewRecorder()
			WriteError(w, httptest.NewRequest("GET", "/", nil), fmt.Errorf("wrapped: %w", e))
			if w.Code != status || !strings.Contains(w.Body.String(), typ) {
				t.Fatal(w)
			}
		})
	}
	w := httptest.NewRecorder()
	WriteError(w, httptest.NewRequest("GET", "/", nil), errors.New("secret"))
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w)
	}
}

type typedError struct{ secret string }

func (e *typedError) Error() string { return e.secret }
func TestTypedError(t *testing.T) {
	Register(&typedError{}, "validation", 422)
	w := httptest.NewRecorder()
	WriteError(w, httptest.NewRequest("GET", "/", nil), fmt.Errorf("wrap: %w", &typedError{"secret"}))
	if w.Code != 422 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w)
	}
}
func TestLimiter(t *testing.T) {
	clk := clocktest.NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	l := NewTokenBucket(0.5, 2, clk).(*tokenBucket)
	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatal("burst")
		}
	}
	if ok, retry := l.Allow("a"); ok || retry != 2*time.Second {
		t.Fatal(ok, retry)
	}
	clk.Advance(time.Second)
	if ok, retry := l.Allow("a"); ok || retry != time.Second {
		t.Fatal(ok, retry)
	}
	clk.Advance(time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("refill")
	}
	clk.Advance(10 * time.Minute)
	l.Allow("b")
	if _, ok := l.keys["a"]; ok {
		t.Fatal("not evicted")
	}
	handler := RateLimit(l, func(*http.Request) string { return "c" })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", "/", nil))
		if i == 2 && (w.Code != 429 || w.Header().Get("Retry-After") != "2") {
			t.Fatal(w)
		}
	}
}
func TestRouter(t *testing.T) {
	var logs bytes.Buffer
	reg := prometheus.NewRegistry()
	router := NewRouter(Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil)), BodyLimit: 4, Production: true, Registerer: reg})
	router.HandleFunc("GET /phone/{phone}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	router.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) { panic("sensitive-value") })
	called := false
	router.HandleFunc("POST /body", func(http.ResponseWriter, *http.Request) { called = true })
	for _, tc := range []struct {
		method, path, body, id string
		status                 int
	}{{"GET", "/phone/+6281234567789?token=secret", "", "trusted-ID", 204}, {"GET", "/panic", "", "bad id", 500}, {"POST", "/body", "12345", "", 413}, {"GET", "/absent", "", "", 404}} {
		w := httptest.NewRecorder()
		q := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		q.ContentLength = -1
		q.Header.Set("X-Request-Id", tc.id)
		router.ServeHTTP(w, q)
		if w.Code != tc.status || !requestIDPattern.MatchString(w.Header().Get("X-Request-Id")) || w.Header().Get("Strict-Transport-Security") == "" || w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal(w)
		}
		if tc.id == "trusted-ID" && w.Header().Get("X-Request-Id") != tc.id {
			t.Fatal("id not preserved")
		}
	}
	if called {
		t.Fatal("oversized request reached handler")
	}
	for _, secret := range []string{"6281234567789", "token=secret", "sensitive-value"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("log leaked", secret)
		}
	}
	for _, field := range []string{"request_id", "method", "route", "status", "duration"} {
		if !strings.Contains(logs.String(), field) {
			t.Fatal(field)
		}
	}
	metrics, err := reg.Gather()
	if err != nil || len(metrics) != 1 {
		t.Fatal(metrics, err)
	}
	dev := NewRouter(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	w := httptest.NewRecorder()
	dev.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("dev HSTS")
	}
}
func TestMaskPhone(t *testing.T) {
	if got := MaskPhone("+6281234567789"); got != "+62812****789" {
		t.Fatal(got)
	}
	for _, p := range []string{"", "123", "+secret123456"} {
		if MaskPhone(p) != "****" {
			t.Fatal(p)
		}
	}
}
