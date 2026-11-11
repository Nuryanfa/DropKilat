// Package clock supplies time to application services.
package clock

import "time"

type Clock interface{ Now() time.Time }
type Real struct{}

func (Real) Now() time.Time { return time.Now().UTC() }
