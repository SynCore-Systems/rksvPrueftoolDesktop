// Package engine installs the official BMF verification tools
// (regkassen-verification 1.1.1). They are published by the BMF without a
// license, so the public build downloads them from the official release (or
// imports a manually downloaded copy) instead of redistributing them; every
// copy is checked against the pinned SHA-256 before use.
package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SynCore-Systems/rksvPrueftoolDesktop/assets"
)

const (
	Version     = "1.1.1"
	ZipName     = "regkassen-verification-1.1.1.zip"
	DownloadURL = "https://github.com/BMF-RKSV-Technik/at-registrierkassen-mustercode/releases/download/V1.1.1/" + ZipName
	ReleaseURL  = "https://github.com/BMF-RKSV-Technik/at-registrierkassen-mustercode/releases/tag/V1.1.1"
	SHA256      = "2c1d65b7b0024262e36ebbfba296c47773c68c7000a6f33c36f80b1215e414b6"

	DEPJar      = "regkassen-verification-depformat-1.1.1.jar"
	ReceiptsJar = "regkassen-verification-receipts-1.1.1.jar"

	maxZipSize   = 64 << 20
	topLevelDir  = "regkassen-verification-1.1.1/"
	completeFile = ".complete"
)

// Dir returns the engine directory, relative to the data directory.
func Dir() string { return filepath.Join("engine", Version) }

// Bundled reports whether the engine is embedded in this build.
func Bundled() bool { return len(assets.BMFEngine) > 0 }

// Installed reports whether a verified engine is present in dataDir.
func Installed(dataDir string) bool {
	dir := filepath.Join(dataDir, Dir())
	for _, f := range []string{completeFile, DEPJar, ReceiptsJar} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

// InstallBundled installs the embedded engine (internal build).
func InstallBundled(dataDir string) error {
	if !Bundled() {
		return fmt.Errorf("engine is not bundled in this build")
	}
	return Install(dataDir, assets.BMFEngine)
}

// Download fetches the official release zip.
func Download(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DownloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Download fehlgeschlagen: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxZipSize))
}

// Install verifies the release zip against the pinned checksum and unpacks
// it into dataDir/engine/<version>, replacing any previous installation.
func Install(dataDir string, zipData []byte) error {
	sum := sha256.Sum256(zipData)
	if got := hex.EncodeToString(sum[:]); got != SHA256 {
		return fmt.Errorf("Prüfsumme stimmt nicht – dies ist nicht die offizielle Datei %s (SHA-256 %s)", ZipName, got)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	final := filepath.Join(dataDir, Dir())
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(final), ".install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, topLevelDir)
		if name == "" || f.FileInfo().IsDir() {
			continue
		}
		target := filepath.Join(tmp, filepath.FromSlash(name))
		if !strings.HasPrefix(target, tmp+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in zip: %s", f.Name)
		}
		if err := extractFile(f, target); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, completeFile), []byte(SHA256), 0o644); err != nil {
		return err
	}
	if err := os.RemoveAll(final); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func extractFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
