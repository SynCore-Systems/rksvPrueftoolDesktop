// Package verify interprets the output of the BMF RKSV verification JARs.
//
// The JARs exit 0 even when a receipt or DEP fails verification, so the exit
// code alone is not a verdict; the verdict and the human-readable failure
// causes are derived from the per-check markers and JSON result trees the
// tools print.
package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Verdicts.
const (
	Pass    = "pass"
	Fail    = "fail"
	Error   = "error"
	Unknown = "unknown"
)

const (
	maxReasons = 5
	// Per-receipt DEP result files can number in the thousands; the failure
	// messages repeat, so a bounded sample is enough.
	maxDEPResultFiles = 500
)

var (
	validationMarkerRE = regexp.MustCompile(`validation # \d+:\s*(PASS|FAIL)`)
	stateRE            = regexp.MustCompile(`"verificationState"\s*:\s*"(PASS|FAIL)"`)
)

// DetermineVerdict derives the RKSV verdict from the tool output: any FAIL
// marker or result state means the overall check failed.
func DetermineVerdict(output string, exitCode int) string {
	if exitCode != 0 {
		return Error
	}
	var results []string
	for _, m := range validationMarkerRE.FindAllStringSubmatch(output, -1) {
		results = append(results, m[1])
	}
	for _, m := range stateRE.FindAllStringSubmatch(output, -1) {
		results = append(results, m[1])
	}
	if len(results) == 0 {
		return Unknown
	}
	for _, r := range results {
		if r == "FAIL" {
			return Fail
		}
	}
	return Pass
}

// node is one entry of a BMF verification result tree.
type node struct {
	State    string `json:"verificationState"`
	Name     string `json:"verificationName"`
	Message  string `json:"verificationResultDetailedMessage"`
	Children []node `json:"verificationResultList"`
}

type failure struct {
	msg      string
	fallback bool
}

// resultTrees returns the JSON result tree printed after each
// "validation # N: X" marker (directly or after a "DETAILS:" line).
func resultTrees(output string) []node {
	var trees []node
	for _, loc := range validationMarkerRE.FindAllStringIndex(output, -1) {
		start := strings.IndexByte(output[loc[1]:], '{')
		if start < 0 {
			continue
		}
		var n node
		if err := json.NewDecoder(strings.NewReader(output[loc[1]+start:])).Decode(&n); err == nil {
			trees = append(trees, n)
		}
	}
	return trees
}

// collectFailures returns the messages of the deepest FAIL nodes that carry
// a detailed message. If no such node exists below a failed check, its own
// message is used, and failing that, the check's name (fallback).
func collectFailures(n node) []failure {
	if n.State != "FAIL" {
		return nil
	}
	var found []failure
	for _, c := range n.Children {
		found = append(found, collectFailures(c)...)
	}
	for _, f := range found {
		if !f.fallback {
			return found
		}
	}
	if n.Message != "" {
		return []failure{{n.Message, false}}
	}
	if len(found) == 0 && n.Name != "" {
		return []failure{{n.Name, true}}
	}
	return found
}

// ExtractReasons summarises the result. On failure: the specific causes
// (deepest failed checks, de-duplicated, at most 5); otherwise the tool's
// first summary message.
func ExtractReasons(output, outputDir, jobType, verdict string) []string {
	trees := resultTrees(output)
	if verdict != Fail {
		for _, t := range trees {
			if t.Message != "" {
				return []string{t.Message}
			}
		}
		return []string{}
	}

	var failures []failure
	if jobType == "dep" {
		// The DEP verdict in stdout only says "receipt list failed"; the
		// cause is in the per-receipt result files.
		files, _ := filepath.Glob(filepath.Join(outputDir, "*_dep.json"))
		sort.Strings(files)
		if len(files) > maxDEPResultFiles {
			files = files[:maxDEPResultFiles]
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			var n node
			if json.Unmarshal(data, &n) == nil {
				failures = append(failures, collectFailures(n)...)
			}
		}
	}
	for _, t := range trees {
		failures = append(failures, collectFailures(t)...)
	}

	var real, all []string
	for _, f := range failures {
		all = append(all, f.msg)
		if !f.fallback {
			real = append(real, f.msg)
		}
	}
	if len(real) > 0 {
		all = real
	}
	reasons := []string{}
	seen := map[string]bool{}
	for _, m := range all {
		if !seen[m] {
			seen[m] = true
			reasons = append(reasons, m)
		}
	}
	if len(reasons) > maxReasons {
		reasons = reasons[:maxReasons]
	}
	return reasons
}
