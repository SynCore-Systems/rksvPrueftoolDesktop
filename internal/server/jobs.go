package server

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/engine"
	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/platform"
	"github.com/SynCore-Systems/rksvPrueftoolDesktop/internal/verify"
)

// Present in a job dir while its verification runs; cleanup skips it.
const runningMarker = ".running"

const previewLines = 100

const defaultTimezone = "Europe/Vienna"

// RKSV receipt timestamps are naive local time; the Java validator reads them
// in its JVM timezone, so it must match the zone the receipts were created in.
var allowedTimezones = map[string]bool{
	"Europe/Vienna": true, "Europe/Berlin": true, "Europe/Zurich": true, "Europe/Bratislava": true,
	"Europe/Budapest": true, "Europe/Prague": true, "Europe/Rome": true, "Europe/Ljubljana": true,
	"Europe/London": true, "UTC": true,
}

func normalizeTimezone(tz string) string {
	if allowedTimezones[tz] {
		return tz
	}
	return defaultTimezone
}

// inputError is invalid user input, reported as HTTP 400.
type inputError string

func (e inputError) Error() string { return string(e) }

// Meta is stored as meta.json per job (same schema as rksvWebPrueftool).
type Meta struct {
	JobID     string   `json:"jobId"`
	Timestamp string   `json:"timestamp"`
	Type      string   `json:"type"`
	InputFile string   `json:"inputFile"`
	Timezone  string   `json:"timezone"`
	ExitCode  *int     `json:"exitCode"`
	Verdict   string   `json:"verdict"`
	Reason    string   `json:"reason"`
	Reasons   []string `json:"reasons"`
	Lines     int      `json:"lines"`
	Chars     int      `json:"chars"`
}

func writeMeta(jobDir string, m Meta) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := filepath.Join(jobDir, "meta.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(jobDir, "meta.json"))
}

func saveUpload(fh *multipart.FileHeader, target string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func formFile(r *http.Request, name string) *multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}
	if files := r.MultipartForm.File[name]; len(files) > 0 && files[0].Filename != "" {
		return files[0]
	}
	return nil
}

func parseJSONObject(data []byte) error {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return inputError("Ungültiges JSON im kryptographischen Material: " + err.Error())
	}
	if obj == nil {
		return inputError("Ungültiges JSON im kryptographischen Material: Objekt erwartet")
	}
	return nil
}

// saveCryptoMaterial stores the crypto container from the uploaded file, the
// JSON built by the page, or the individual form fields.
func saveCryptoMaterial(r *http.Request, target string) error {
	if fh := formFile(r, "cryptoFile"); fh != nil {
		f, err := fh.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, 16<<20))
		f.Close()
		if err != nil {
			return err
		}
		if err := parseJSONObject(data); err != nil {
			return err
		}
		// Keep the uploaded bytes untouched; the BMF tool is the judge.
		return os.WriteFile(target, data, 0o644)
	}
	if r.MultipartForm != nil {
		if v, ok := r.MultipartForm.Value["cryptoData"]; ok && len(v) > 0 {
			if err := parseJSONObject([]byte(v[0])); err != nil {
				return err
			}
			return os.WriteFile(target, []byte(v[0]), 0o644)
		}
	}
	fields := map[string]string{}
	for _, k := range []string{"base64key", "id", "sigType", "cert"} {
		fields[k] = strings.TrimSpace(r.FormValue(k))
		if fields[k] == "" {
			return inputError("Kryptographisches Material fehlt (Datei oder alle Felder)")
		}
	}
	data, err := json.Marshal(map[string]any{
		"base64AESKey": fields["base64key"],
		"certificateOrPublicKeyMap": map[string]any{
			fields["id"]: map[string]string{
				"id":                              fields["id"],
				"signatureDeviceType":             fields["sigType"],
				"signatureCertificateOrPublicKey": fields["cert"],
			},
		},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

func (s *Server) handleVerifyDEP(w http.ResponseWriter, r *http.Request) {
	s.handleVerification(w, r, "dep", engine.DEPJar, func(jobDir string) (string, string, error) {
		fh := formFile(r, "depFile")
		if fh == nil {
			return "", "", inputError("DEP-Export Datei fehlt")
		}
		if err := saveUpload(fh, filepath.Join(jobDir, "dep-export.json")); err != nil {
			return "", "", err
		}
		return "dep-export.json", fh.Filename, nil
	})
}

func (s *Server) handleVerifyReceipts(w http.ResponseWriter, r *http.Request) {
	s.handleVerification(w, r, "receipts", engine.ReceiptsJar, func(jobDir string) (string, string, error) {
		target := filepath.Join(jobDir, "qr-code-rep.json")
		if fh := formFile(r, "qrFile"); fh != nil {
			if err := saveUpload(fh, target); err != nil {
				return "", "", err
			}
			return "qr-code-rep.json", fh.Filename, nil
		}
		// One QR code string per line.
		codes := []string{}
		for _, line := range strings.Split(r.FormValue("qrString"), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				codes = append(codes, line)
			}
		}
		if len(codes) == 0 {
			return "", "", inputError("QR-Code Datei oder QR-Code String fehlt")
		}
		data, _ := json.Marshal(codes)
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return "", "", err
		}
		return "qr-code-rep.json", "manual-input", nil
	})
}

// handleVerification runs the common job lifecycle: create the job dir, store
// input and crypto material, run the JAR. Input errors discard the job; later
// errors are recorded in meta.json so the job shows up in the history.
func (s *Server) handleVerification(w http.ResponseWriter, r *http.Request, jobType, jar string,
	prepareInput func(jobDir string) (inputName, inputFilename string, err error)) {

	javaPath, ok := s.ready()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "Das Prüfmodul ist noch nicht bereit")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "Datei zu groß (max. 2 GB)")
			return
		}
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage: "+err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()

	id := newJobID()
	jobDir := s.jobDir(id)
	if err := os.MkdirAll(filepath.Join(jobDir, "output"), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	marker := filepath.Join(jobDir, runningMarker)
	os.WriteFile(marker, nil, 0o644)
	defer os.Remove(marker)

	inputName, inputFilename, err := prepareInput(jobDir)
	if err == nil {
		err = saveCryptoMaterial(r, filepath.Join(jobDir, "cryptographicMaterialContainer.json"))
	}
	var inErr inputError
	if errors.As(err, &inErr) {
		os.RemoveAll(jobDir)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	meta := Meta{
		JobID:     id,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      jobType,
		InputFile: inputFilename,
		Timezone:  normalizeTimezone(r.FormValue("timezone")),
		Reasons:   []string{},
	}
	if err != nil {
		s.recordError(w, jobDir, meta, http.StatusInternalServerError, err.Error())
		return
	}
	s.runVerification(w, r.Context(), javaPath, jar, id, inputName, meta)
}

func (s *Server) recordError(w http.ResponseWriter, jobDir string, meta Meta, status int, msg string) {
	log.Printf("job %s: %s", meta.JobID, msg)
	meta.Verdict, meta.Reason, meta.Reasons = verify.Error, msg, []string{msg}
	if err := writeMeta(jobDir, meta); err != nil {
		log.Printf("job %s: writing meta: %v", meta.JobID, err)
	}
	writeJSON(w, status, map[string]string{"error": msg, "jobId": meta.JobID})
}

func (s *Server) runVerification(w http.ResponseWriter, reqCtx context.Context, javaPath, jar, id, inputName string, meta Meta) {
	jobDir := s.jobDir(id)

	select {
	case s.sem <- struct{}{}:
	case <-reqCtx.Done():
		os.RemoveAll(jobDir)
		return
	}
	defer func() { <-s.sem }()

	// Paths relative to the data dir and the 8.3 form of the absolute ones
	// keep the Java 8 command line ASCII-only (user names may have umlauts).
	rel := func(name string) string { return filepath.Join("jobs", id, name) }
	args := []string{
		"-Xmx2g", "-Dfile.encoding=UTF-8", "-Duser.timezone=" + meta.Timezone,
		"-jar", filepath.Join(engine.Dir(), jar), "-v", "-f",
		"-i", rel(inputName), "-c", rel("cryptographicMaterialContainer.json"), "-o", rel("output"),
	}
	ctx, cancel := context.WithTimeout(s.ctx, verifyTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, platform.ShortPath(javaPath), args...)
	cmd.Dir = platform.ShortPath(s.dataDir)
	platform.HideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	log.Printf("job %s: running %s", id, strings.Join(args, " "))
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		s.recordError(w, jobDir, meta, http.StatusGatewayTimeout,
			fmt.Sprintf("Zeitüberschreitung der Prüfung (%d Minuten)", int(verifyTimeout.Minutes())))
		return
	}
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			s.recordError(w, jobDir, meta, http.StatusInternalServerError, "Java konnte nicht gestartet werden: "+err.Error())
			return
		}
		exitCode = exitErr.ExitCode()
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[STDERR]\n" + stderr.String()
	}
	output = strings.ReplaceAll(output, "\r\n", "\n")
	if err := os.WriteFile(filepath.Join(jobDir, "output", "verification.log"), []byte(output), 0o644); err != nil {
		s.recordError(w, jobDir, meta, http.StatusInternalServerError, err.Error())
		return
	}

	lines := splitLines(output)
	verdict := verify.DetermineVerdict(output, exitCode)
	reasons := verify.ExtractReasons(output, filepath.Join(jobDir, "output"), meta.Type, verdict)
	meta.ExitCode = &exitCode
	meta.Verdict = verdict
	meta.Reasons = reasons
	if len(reasons) > 0 {
		meta.Reason = reasons[0]
	}
	meta.Lines = len(lines)
	meta.Chars = len([]rune(output))
	if err := writeMeta(jobDir, meta); err != nil {
		log.Printf("job %s: writing meta: %v", id, err)
	}
	log.Printf("job %s: exit %d, verdict %s", id, exitCode, verdict)

	preview := lines
	if len(preview) > previewLines {
		preview = preview[:previewLines]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jobId":       id,
		"exitCode":    exitCode,
		"success":     exitCode == 0,
		"verdict":     verdict,
		"reason":      meta.Reason,
		"reasons":     reasons,
		"timezone":    meta.Timezone,
		"lines":       meta.Lines,
		"chars":       meta.Chars,
		"preview":     strings.Join(preview, "\n"),
		"logUrl":      "/api/log/" + id,
		"downloadUrl": "/api/download/" + id,
	})
}

// splitLines splits like Python's str.splitlines() for "\n"-normalized text.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func (s *Server) logPath(id string) string {
	return filepath.Join(s.jobDir(id), "output", "verification.log")
}

func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	f, err := os.Open(s.logPath(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "Log nicht gefunden")
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="verification-%s.log"`, id))
	http.ServeContent(w, r, "", info.ModTime(), f)
}

func (s *Server) handleLogPage(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	start, err1 := strconv.Atoi(r.URL.Query().Get("start"))
	limit, err2 := strconv.Atoi(r.URL.Query().Get("limit"))
	if r.URL.Query().Get("start") == "" {
		start, err1 = 0, nil
	}
	if r.URL.Query().Get("limit") == "" {
		limit, err2 = 1000, nil
	}
	if err1 != nil || err2 != nil || start < 0 || limit < 1 || limit > 5000 {
		writeError(w, http.StatusBadRequest, "Ungültige start/limit Parameter")
		return
	}
	f, err := os.Open(s.logPath(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "Log nicht gefunden")
		return
	}
	defer f.Close()

	var chunk strings.Builder
	total := 0
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			if total >= start && total < start+limit {
				chunk.WriteString(line)
			}
			total++
		}
		if err != nil {
			break
		}
	}
	end := min(total, start+limit)
	if start > total {
		end = start
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"start": start, "end": end, "total": total, "chunk": chunk.String(),
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id, ok := validJobID(w, r)
	if !ok {
		return
	}
	jobDir := s.jobDir(id)
	if _, err := os.Stat(jobDir); err != nil {
		writeError(w, http.StatusNotFound, "Ergebnisse nicht gefunden")
		return
	}
	if _, err := os.Stat(filepath.Join(jobDir, "meta.json")); err != nil || isRunning(jobDir) {
		writeError(w, http.StatusConflict, "Die Prüfung läuft noch")
		return
	}
	zipPath := filepath.Join(s.jobsDir, id+"-results.zip")
	if _, err := os.Stat(zipPath); err != nil {
		if err := buildZip(s.jobsDir, id, zipPath); err != nil {
			writeError(w, http.StatusInternalServerError, "ZIP konnte nicht erstellt werden: "+err.Error())
			return
		}
	}
	f, err := os.Open(zipPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="results-%s.zip"`, id))
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// buildZip archives jobs/<id> (entries prefixed "<id>/") into a temp file and
// renames it into place, so a concurrent request never sees a partial file.
func buildZip(jobsDir, id, zipPath string) error {
	tmp, err := os.CreateTemp(jobsDir, id+"-*.zip.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	zw := zip.NewWriter(tmp)
	root := filepath.Join(jobsDir, id)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == runningMarker {
			return err
		}
		rel, err := filepath.Rel(jobsDir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
	if err == nil {
		err = zw.Close()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), zipPath)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.jobsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jobs := []json.RawMessage{}
	var stamps []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.jobsDir, e.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var m struct {
			Timestamp string `json:"timestamp"`
		}
		if json.Unmarshal(data, &m) != nil {
			log.Printf("skipping corrupt meta.json in %s", e.Name())
			continue
		}
		jobs = append(jobs, data)
		stamps = append(stamps, m.Timestamp)
	}
	idx := make([]int, len(jobs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return stamps[idx[a]] > stamps[idx[b]] })
	sorted := make([]json.RawMessage, len(jobs))
	for i, j := range idx {
		sorted[i] = jobs[j]
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": sorted})
}
