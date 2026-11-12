package httpx

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"github.com/Nuryanfa/DropKilat/internal/platform/clock"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// MaskPhone preserves only the country/prefix and last three digits of valid E.164 numbers.
func MaskPhone(phone string) string {
	if len(phone) < 10 || len(phone) > 16 || phone[0] != '+' {
		return "****"
	}
	for _, c := range phone[1:] {
		if c < '0' || c > '9' {
			return "****"
		}
	}
	return phone[:6] + "****" + phone[len(phone)-3:]
}

type Options struct {
	Logger     *slog.Logger
	Clock      clock.Clock
	Production bool
	BodyLimit  int64
	Limiter    Limiter
	Key        func(*http.Request) string
	Registerer prometheus.Registerer
}
type Router struct {
	mux     *http.ServeMux
	handler http.Handler
}

func (r *Router) Handle(pattern string, h http.Handler)            { r.mux.Handle(pattern, h) }
func (r *Router) HandleFunc(pattern string, h http.HandlerFunc)    { r.Handle(pattern, h) }
func (r *Router) ServeHTTP(w http.ResponseWriter, q *http.Request) { r.handler.ServeHTTP(w, q) }

// NewRouter owns its histogram; supply an isolated registry or register once at startup.
// Forwarded headers are deliberately ignored: callers must explicitly supply trusted keys.
func NewRouter(o Options) *Router {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Clock == nil {
		o.Clock = clock.Real{}
	}
	if o.BodyLimit <= 0 {
		o.BodyLimit = 1 << 20
	}
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "dropkilat_http_request_duration_seconds", Help: "HTTP response latency."}, []string{"method", "route", "status"})
	if o.Registerer != nil {
		o.Registerer.MustRegister(histogram)
	}
	mux := http.NewServeMux()
	var inner http.Handler = mux
	if o.Limiter != nil {
		if o.Key == nil {
			panic("rate limit key required")
		}
		inner = RateLimit(o.Limiter, o.Key)(inner)
	}
	r := &Router{mux: mux}
	r.handler = http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		id := q.Header.Get("X-Request-Id")
		if !requestIDPattern.MatchString(id) {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				panic(err)
			}
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-Id", id)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if o.Production {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		sw := &statusWriter{ResponseWriter: w}
		start := o.Clock.Now()
		defer func() {
			if recover() != nil {
				o.Logger.Error("handler panic", "request_id", id, "stack", string(debug.Stack()))
				if sw.status == 0 {
					WriteProblem(sw, q, Problem{Type: "internal", Status: 500})
				}
			}
			status := sw.status
			if status == 0 {
				status = 200
			}
			route := q.Pattern
			if route == "" {
				route = "unmatched"
			}
			method := q.Method
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
			default:
				method = "other"
			}
			duration := o.Clock.Now().Sub(start)
			o.Logger.Info("http request", "request_id", id, "method", method, "route", route, "status", status, "duration", duration)
			histogram.WithLabelValues(method, route, strconv.Itoa(status)).Observe(duration.Seconds())
		}()
		// Read a bounded body before invoking business handlers, including chunked requests.
		if q.Body != nil {
			body, err := io.ReadAll(http.MaxBytesReader(sw, q.Body, o.BodyLimit))
			_ = q.Body.Close()
			if err != nil {
				status := 400
				typ := "validation"
				if _, ok := err.(*http.MaxBytesError); ok {
					status = 413
					typ = "body-too-large"
				}
				WriteProblem(sw, q, Problem{Type: typ, Status: status})
				return
			}
			q.Body = io.NopCloser(bytes.NewReader(body))
		}
		inner.ServeHTTP(sw, q)
	})
	return r
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
