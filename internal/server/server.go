// Package server is the local HTTP backend of the desktop app. It mirrors the
// API of rksvWebPrueftool (web/app.py), so the same page drives it, and adds
// endpoints for setting up the BMF engine. It only listens on 127.0.0.1 and
// every request must carry the per-start session token.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/engine"
	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/jre"
	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/platform"
)

const (
	maxUploadSize = 2 << 30
	verifyTimeout = 30 * time.Minute
	cookieName    = "rksv_session"
	// Destructive endpoints additionally require this header (as in the web app).
	csrfHeader = "X-Requested-With"
	csrfValue  = "rksv"
)

var jobIDRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Setup states reported by /api/status.
const (
	statePreparing  = "preparing"
	stateReady      = "ready"
	stateMissing    = "missing"
	stateInstalling = "installing"
	stateError      = "error"
)

// Server is the local backend.
type Server struct {
	ctx     context.Context
	dataDir string
	jobsDir string
	token   string
	ui      fs.FS

	// One verification at a time: each runs a JVM with -Xmx2g.
	sem chan struct{}

	mu           sync.Mutex
	javaPath     string
	runtimeState string
	runtimeErr   string
	engineState  string
	engineErr    string
}

// New creates the server. ctx bounds running verifications (cancelled on
// exit). ui holds upload.html and the static files.
func New(ctx context.Context, dataDir string, ui fs.FS) (*Server, error) {
	jobsDir := filepath.Join(dataDir, "jobs")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		return nil, err
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	return &Server{
		ctx:          ctx,
		dataDir:      dataDir,
		jobsDir:      jobsDir,
		token:        hex.EncodeToString(token),
		ui:           ui,
		sem:          make(chan struct{}, 1),
		runtimeState: statePreparing,
		engineState:  statePreparing,
	}, nil
}

// Token is the session token the window must open the page with.
func (s *Server) Token() string { return s.token }

// Prepare unpacks the Java runtime and installs the bundled engine if
// needed. Run it in the background; the page shows progress via /api/status.
func (s *Server) Prepare() {
	javaPath, err := jre.Ensure(s.dataDir)
	s.mu.Lock()
	if err != nil {
		log.Printf("runtime: %v", err)
		s.runtimeState, s.runtimeErr = stateError, err.Error()
	} else {
		s.javaPath, s.runtimeState = javaPath, stateReady
		log.Printf("runtime: %s", javaPath)
	}
	s.mu.Unlock()

	switch {
	case engine.Installed(s.dataDir):
		s.setEngine(stateReady, "")
	case engine.Bundled():
		s.setEngine(stateInstalling, "")
		if err := engine.InstallBundled(s.dataDir); err != nil {
			s.setEngine(stateError, err.Error())
		} else {
			s.setEngine(stateReady, "")
		}
	default:
		s.setEngine(stateMissing, "")
	}
}

func (s *Server) setEngine(state, errMsg string) {
	s.mu.Lock()
	s.engineState, s.engineErr = state, errMsg
	s.mu.Unlock()
	if errMsg != "" {
		log.Printf("engine: %s: %s", state, errMsg)
	}
}

// ready returns the java path once runtime and engine are usable.
func (s *Server) ready() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.javaPath, s.runtimeState == stateReady && s.engineState == stateReady
}

// Handler returns the HTTP handler with session-token protection.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.Handle("GET /static/", http.FileServerFS(s.ui))
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/engine/download", s.handleEngineDownload)
	mux.HandleFunc("POST /api/engine/import", s.handleEngineImport)
	mux.HandleFunc("POST /api/open-external", s.handleOpenExternal)
	mux.HandleFunc("POST /api/verify/dep", s.handleVerifyDEP)
	mux.HandleFunc("POST /api/verify/receipts", s.handleVerifyReceipts)
	mux.HandleFunc("GET /api/log/{id}", s.handleLog)
	mux.HandleFunc("GET /api/log/{id}/page", s.handleLogPage)
	mux.HandleFunc("GET /api/download/{id}", s.handleDownload)
	mux.HandleFunc("GET /api/jobs", s.handleJobs)
	mux.HandleFunc("POST /api/cleanup", s.handleCleanup)
	return s.authenticate(mux)
}

// authenticate admits requests carrying the session cookie. The window opens
// "/?t=<token>", which sets the cookie (SameSite=Strict, so other sites and
// local processes without the token can't use the API).
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" && r.URL.Query().Get("t") == s.token {
			http.SetCookie(w, &http.Cookie{
				Name: cookieName, Value: s.token, Path: "/",
				HttpOnly: true, SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if c, err := r.Cookie(cookieName); err != nil || c.Value != s.token {
			http.Error(w, "Bitte das RKSV Prüftool über die Programmdatei starten.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	page, err := fs.ReadFile(s.ui, "upload.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(page)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"ready": s.runtimeState == stateReady && s.engineState == stateReady,
		"runtime": map[string]string{
			"state": s.runtimeState, "error": s.runtimeErr,
		},
		"engine": map[string]any{
			"state": s.engineState, "error": s.engineErr,
			"bundled": engine.Bundled(), "version": engine.Version,
			"zipName": engine.ZipName, "releaseUrl": engine.ReleaseURL,
		},
	})
}

// beginEngineInstall marks the engine as installing unless an install is
// already running or the engine is ready.
func (s *Server) beginEngineInstall() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.engineState == stateInstalling || s.engineState == stateReady {
		return false
	}
	s.engineState, s.engineErr = stateInstalling, ""
	return true
}

// finishEngineInstall records the install result; failures are answered
// with failStatus (502 for a failed download, 422 for a rejected file).
func (s *Server) finishEngineInstall(w http.ResponseWriter, err error, failStatus int) {
	if err != nil {
		s.setEngine(stateError, err.Error())
		writeError(w, failStatus, "Installation des Prüfmoduls fehlgeschlagen: "+err.Error())
		return
	}
	s.setEngine(stateReady, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleEngineDownload(w http.ResponseWriter, r *http.Request) {
	if !s.beginEngineInstall() {
		writeError(w, http.StatusConflict, "Prüfmodul wird bereits installiert oder ist bereits installiert")
		return
	}
	data, err := engine.Download(r.Context())
	if err == nil {
		err = engine.Install(s.dataDir, data)
	}
	s.finishEngineInstall(w, err, http.StatusBadGateway)
}

func (s *Server) handleEngineImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<20)
	file, _, err := r.FormFile("engineZip")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Bitte die Datei "+engine.ZipName+" auswählen")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.beginEngineInstall() {
		writeError(w, http.StatusConflict, "Prüfmodul wird bereits installiert oder ist bereits installiert")
		return
	}
	s.finishEngineInstall(w, engine.Install(s.dataDir, data), http.StatusUnprocessableEntity)
}

func (s *Server) handleOpenExternal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "Ungültige Adresse")
		return
	}
	if err := platform.OpenURL(u.String()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func newJobID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func (s *Server) jobDir(id string) string { return filepath.Join(s.jobsDir, id) }

func isRunning(jobDir string) bool {
	_, err := os.Stat(filepath.Join(jobDir, runningMarker))
	return err == nil
}

func validJobID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !jobIDRE.MatchString(id) {
		writeError(w, http.StatusBadRequest, "Ungültige Job-ID")
		return "", false
	}
	return id, true
}

func (s *Server) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(csrfHeader) != csrfValue {
		writeError(w, http.StatusForbidden, "Forbidden")
		return
	}
	entries, err := os.ReadDir(s.jobsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	skipped := 0
	for _, e := range entries {
		p := filepath.Join(s.jobsDir, e.Name())
		if e.IsDir() && isRunning(p) {
			skipped++
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			log.Printf("cleanup: %v", err)
		}
	}
	msg := "Alle Daten gelöscht"
	if skipped > 0 {
		msg = fmt.Sprintf("Alle Daten gelöscht, außer %d laufende(n) Prüfung(en)", skipped)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": msg})
}

