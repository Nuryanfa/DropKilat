package httpx

import (
	"github.com/Nuryanfa/DropKilat/internal/platform/clock"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Limiter interface {
	Allow(string) (bool, time.Duration)
}
type bucket struct {
	tokens float64
	at     time.Time
}
type tokenBucket struct {
	mu    sync.Mutex
	rate  float64
	burst int
	clk   clock.Clock
	keys  map[string]bucket
	swept time.Time
}

func NewTokenBucket(rate float64, burst int, clk clock.Clock) Limiter {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) || burst < 1 || clk == nil {
		panic("invalid token bucket configuration")
	}
	return &tokenBucket{rate: rate, burst: burst, clk: clk, keys: make(map[string]bucket)}
}
func (l *tokenBucket) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clk.Now()
	if now.Sub(l.swept) >= time.Minute {
		for k, b := range l.keys {
			if now.Sub(b.at) >= 10*time.Minute {
				delete(l.keys, k)
			}
		}
		l.swept = now
	}
	b, ok := l.keys[key]
	if !ok {
		b = bucket{float64(l.burst), now}
	}
	elapsed := now.Sub(b.at).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	b.tokens = math.Min(float64(l.burst), b.tokens+elapsed*l.rate)
	b.at = now
	allowed := b.tokens >= 1
	var retry time.Duration
	if allowed {
		b.tokens--
	} else {
		retry = time.Duration(math.Ceil((1 - b.tokens) / l.rate * float64(time.Second)))
	}
	l.keys[key] = b
	return allowed, retry
}
func RateLimit(l Limiter, key func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ok, retry := l.Allow(key(r)); !ok {
				seconds := math.Max(1, math.Ceil(retry.Seconds()))
				w.Header().Set("Retry-After", strconv.FormatFloat(seconds, 'f', 0, 64))
				WriteProblem(w, r, Problem{Type: "rate-limited", Status: 429})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
