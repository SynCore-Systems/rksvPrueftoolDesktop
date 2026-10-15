//go:build windows

// Package window shows the user interface in a native WebView2 window.
package window

import (
	"errors"
	"path/filepath"

	webview2 "github.com/jchv/go-webview2"
)

// IconID is the resource ID of the application icon (see winres/winres.json).
const IconID = 1

// ErrNoWebView2 means the Microsoft Edge WebView2 runtime is missing.
var ErrNoWebView2 = errors.New("WebView2 runtime not available")

// Run opens the window on url and blocks until it is closed.
func Run(url, dataDir string) error {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(dataDir, "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  "RKSV Prüftool",
			Width:  1100,
			Height: 900,
			IconId: IconID,
			Center: true,
		},
	})
	if w == nil {
		return ErrNoWebView2
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	return nil
}
