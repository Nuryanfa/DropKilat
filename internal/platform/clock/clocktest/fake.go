// Package clocktest provides a concurrency-safe controllable clock.
package clocktest

import (
	"sync"
	"time"
)

type Fake struct {
	mu  sync.RWMutex
	now time.Time
}

func NewFake(t time.Time) *Fake         { return &Fake{now: t.UTC()} }
func (f *Fake) Now() time.Time          { f.mu.RLock(); defer f.mu.RUnlock(); return f.now }
func (f *Fake) Advance(d time.Duration) { f.mu.Lock(); defer f.mu.Unlock(); f.now = f.now.Add(d) }
