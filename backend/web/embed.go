package web

import "embed"

// DistFS contains the built frontend assets.
//go:embed all:dist
var DistFS embed.FS
