//go:build !windows

// Package window shows the user interface. Outside Windows there is no
// native window; the app runs headless (see the --serve flag).
package window

import "errors"

// ErrNoWebView2 means no native window is available.
var ErrNoWebView2 = errors.New("native window only available on Windows; use --serve")

// Run is unsupported outside Windows.
func Run(url, dataDir string) error { return ErrNoWebView2 }
