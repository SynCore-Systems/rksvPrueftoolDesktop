//go:build !windows

// Package platform wraps the OS specifics of the desktop app.
package platform

import (
	"fmt"
	"os"
	"os/exec"
)

// SingleInstance is a no-op outside Windows.
func SingleInstance(name string) (release func(), ok bool, err error) {
	return func() {}, true, nil
}

// MessageBox prints the message to stderr.
func MessageBox(title, text string, isError bool) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, text)
}

// OpenURL opens a URL with xdg-open.
func OpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// HideWindow is a no-op outside Windows.
func HideWindow(cmd *exec.Cmd) {}

// ShortPath returns the path unchanged outside Windows.
func ShortPath(path string) string { return path }
