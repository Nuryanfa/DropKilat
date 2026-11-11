package clock

import (
	"testing"
	"time"
)

func TestRealUTC(t *testing.T) {
	if (Real{}).Now().Location() != time.UTC {
		t.Fatal("real clock must return UTC")
	}
}
