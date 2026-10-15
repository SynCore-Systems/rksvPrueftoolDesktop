// RKSV Prüftool Desktop: a self-contained Windows app that validates Austrian
// cash register data (DEP exports, QR codes) with the official BMF tools.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/platform"
	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/server"
	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/window"
	"github.com/SynCore-Systems/rksvPrueftoolDesktop/web"
)

// Set at build time via -ldflags "-X main.version=...".
var version = "dev"

const (
	appTitle       = "RKSV Prüftool"
	webView2URL    = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"
	maxLogFileSize = 5 << 20
)

func main() {
	serve := flag.Bool("serve", false, "headless: serve the UI on 127.0.0.1 and print its URL (no window)")
	dataFlag := flag.String("data", "", "data directory (default: %LOCALAPPDATA%\\SynCore\\RKSV-Prueftool)")
	port := flag.Int("port", 0, "port for the local server (0 = random)")
	flag.Parse()

	if err := run(*serve, *dataFlag, *port); err != nil {
		log.Printf("fatal: %v", err)
		platform.MessageBox(appTitle, "Fehler: "+err.Error(), true)
		os.Exit(1)
	}
}

func run(serve bool, dataDir string, port int) error {
	if dataDir == "" {
		var err error
		if dataDir, err = defaultDataDir(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	setupLogging(dataDir)
	log.Printf("RKSV Prüftool %s starting, data: %s", version, dataDir)

	if !serve {
		release, ok, err := platform.SingleInstance("SynCore-RKSV-Prueftool")
		if err != nil {
			return err
		}
		if !ok {
			platform.MessageBox(appTitle, "Das RKSV Prüftool läuft bereits.", false)
			return nil
		}
		defer release()
	}

	// Cancelling ctx stops running verifications (kills java).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv, err := server.New(ctx, dataDir, web.FS)
	if err != nil {
		return err
	}
	go srv.Prepare()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go httpSrv.Serve(ln)
	defer func() {
		shutdownCtx, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		httpSrv.Shutdown(shutdownCtx)
	}()

	url := fmt.Sprintf("http://%s/?t=%s", ln.Addr(), srv.Token())
	if serve {
		fmt.Println(url)
		<-ctx.Done()
		return nil
	}

	if err := window.Run(url, dataDir); err != nil {
		if errors.Is(err, window.ErrNoWebView2) {
			platform.MessageBox(appTitle, "Die Komponente \"Microsoft Edge WebView2 Runtime\" fehlt.\n\n"+
				"Sie wird jetzt im Browser zum Download geöffnet. Bitte installieren und das Prüftool erneut starten.", true)
			platform.OpenURL(webView2URL)
			return nil
		}
		return err
	}
	return nil
}

func defaultDataDir() (string, error) {
	if runtime.GOOS == "windows" {
		// %LOCALAPPDATA% on Windows.
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "SynCore", "RKSV-Prueftool"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "rksv-prueftool"), nil
}

// setupLogging writes the log to <data>/app.log (the GUI build has no
// console), starting fresh once it exceeds 5 MB.
func setupLogging(dataDir string) {
	path := filepath.Join(dataDir, "app.log")
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogFileSize {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return
	}
	// File first: in the GUI build writes to stderr fail.
	log.SetOutput(io.MultiWriter(f, os.Stderr))
}
