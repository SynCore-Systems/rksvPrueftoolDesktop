// Package web embeds the user interface (adapted from rksvWebPrueftool).
package web

import (
	"embed"
	"io/fs"
)

//go:embed upload.html static
var files embed.FS

// FS serves upload.html at the root and the assets under static/.
var FS fs.FS = files
