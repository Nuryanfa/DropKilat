package clocktest

import (
	"sync"
	"testing"
	"time"
)

func TestFakeConcurrent(t *testing.T) {
	start := time.Date(2026, 11, 11, 12, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	f := NewFake(start)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				f.Advance(time.Second)
				_ = f.Now()
			}
		}()
	}
	wg.Wait()
	if got := f.Now(); !got.Equal(start.Add(10000*time.Second)) || got.Location() != time.UTC {
		t.Fatalf("unexpected time: %v", got)
	}
	f.Advance(-time.Second)
	if !f.Now().Equal(start.Add(9999 * time.Second)) {
		t.Fatal("negative advance failed")
	}
}
