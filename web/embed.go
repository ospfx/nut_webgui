// Package web provides embedded web assets (templates and static files).
package web

import "embed"

// Templates contains all HTML template files.
//
//go:embed templates/*.html
var Templates embed.FS

// Static contains all static web assets.
//
//go:embed static
var Static embed.FS
