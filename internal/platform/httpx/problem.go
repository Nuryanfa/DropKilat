// Package httpx provides the shared HTTP safety stack.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sync"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
type Problem struct {
	Type   string       `json:"type"`
	Title  string       `json:"title"`
	Status int          `json:"status"`
	Detail string       `json:"detail,omitempty"`
	Errors []FieldError `json:"errors,omitempty"`
}

func WriteProblem(w http.ResponseWriter, _ *http.Request, p Problem) {
	if p.Title == "" {
		p.Title = http.StatusText(p.Status)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

type mapping struct {
	target error
	typ    string
	status int
}

var registry struct {
	sync.RWMutex
	entries []mapping
}

// Register is concurrency-safe. Earlier registrations take precedence.
func Register(target error, typ string, status int) {
	if target == nil || typ == "" || status < 200 || status > 599 {
		panic("invalid problem registration")
	}
	registry.Lock()
	defer registry.Unlock()
	registry.entries = append(registry.entries, mapping{target, typ, status})
}
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	registry.RLock()
	entries := append([]mapping(nil), registry.entries...)
	registry.RUnlock()
	for _, m := range entries {
		// Sentinel errors match by identity; concrete domain error types also match via As.
		matched := errors.Is(err, m.target)
		if !matched && reflect.TypeOf(m.target).String() != "*errors.errorString" {
			matched = errors.As(err, reflect.New(reflect.TypeOf(m.target)).Interface())
		}
		if matched {
			WriteProblem(w, r, Problem{Type: m.typ, Status: m.status})
			return
		}
	}
	WriteProblem(w, r, Problem{Type: "internal", Status: 500})
}
