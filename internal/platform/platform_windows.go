//go:build windows

// Package platform wraps the OS specifics of the desktop app.
package platform

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// SingleInstance acquires a named mutex. ok is false if another instance of
// the app already holds it.
func SingleInstance(name string) (release func(), ok bool, err error) {
	namePtr, err := windows.UTF16PtrFromString("Local\\" + name)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, namePtr)
	if err == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return func() { windows.CloseHandle(h) }, true, nil
}

// MessageBox shows a modal message (error icon if isError).
func MessageBox(title, text string, isError bool) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	var flags uint32 = windows.MB_OK | windows.MB_SETFOREGROUND
	if isError {
		flags |= windows.MB_ICONERROR
	} else {
		flags |= windows.MB_ICONINFORMATION
	}
	windows.MessageBox(0, m, t, flags)
}

// OpenURL opens a URL in the default browser.
func OpenURL(url string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	target, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}

// HideWindow keeps a child console process (java.exe) from opening a window.
func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// ShortPath returns the 8.3 short form of an existing path (pure ASCII on
// typical systems), or the path itself if none is available. Java 8 mangles
// non-ASCII characters in paths, e.g. a user name like "Jürgen".
func ShortPath(path string) string {
	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return path
	}
	return windows.UTF16ToString(buf[:n])
}
