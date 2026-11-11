// Package migrations embeds the versioned SQL shipped with the binary.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
