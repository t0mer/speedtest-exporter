// Package web embeds the static dashboard UI.
package web

import "embed"

//go:embed templates static
var FS embed.FS
