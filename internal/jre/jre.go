// Package jre provides the Java 8 runtime that runs the BMF tools. Windows
// builds embed a Temurin JRE and unpack it once into the data directory;
// other builds (used for testing) take java from $RKSV_JAVA or the PATH.
package jre

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SynCore-Systems/rksvPrueftoolDesktop/assets"
)

const completeFile = ".complete"

// Ensure returns the absolute path of the java executable, unpacking the
// embedded runtime on first use.
func Ensure(dataDir string) (string, error) {
	if len(assets.JRE) == 0 {
		if p := os.Getenv("RKSV_JAVA"); p != "" {
			return p, nil
		}
		return exec.LookPath("java")
	}

	sum := sha256.Sum256(assets.JRE)
	key := hex.EncodeToString(sum[:])[:16]
	base := filepath.Join(dataDir, "runtime")
	dir := filepath.Join(base, key)

	if _, err := os.Stat(filepath.Join(dir, completeFile)); err != nil {
		if err := unpack(base, dir); err != nil {
			return "", fmt.Errorf("unpacking Java runtime: %w", err)
		}
	}
	removeOtherVersions(base, key)
	return findJava(dir)
}

func unpack(base, dir string) error {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(base, ".unpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	zr, err := zip.NewReader(bytes.NewReader(assets.JRE), int64(len(assets.JRE)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		target := filepath.Join(tmp, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, tmp+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := extract(f, target); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, completeFile), nil, 0o644); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.Rename(tmp, dir)
}

func extract(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// findJava locates bin/java(.exe) below the unpacked runtime; the archive's
// top-level directory name depends on the JRE version.
func findJava(dir string) (string, error) {
	name := "java"
	if runtime.GOOS == "windows" {
		name = "java.exe"
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*", "bin", name))
	if len(matches) == 0 {
		matches, _ = filepath.Glob(filepath.Join(dir, "bin", name))
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%s not found in %s", name, dir)
	}
	return matches[0], nil
}

// removeOtherVersions deletes runtimes unpacked by older app versions.
func removeOtherVersions(base, keep string) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && e.Name() != keep && !strings.HasPrefix(e.Name(), ".unpack-") {
			os.RemoveAll(filepath.Join(base, e.Name()))
		}
	}
}
