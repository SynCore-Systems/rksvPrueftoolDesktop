package verify

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Synthetic outputs shaped like the BMF tool output (marker line, then the
// JSON result tree, optionally after "DETAILS:").
const receiptPass = `Machine readable code validation # 0: PASS
DETAILS:
{
  "verificationId" : "VERIFICATION_FROM_CASHBOX",
  "verificationState" : "PASS",
  "verificationResultDetailedMessage" : "Der vorliegende Startbeleg wurde gesetzeskonform erstellt."
}
`

const receiptFail = `Machine readable code validation # 0: FAIL
DETAILS:
{
  "verificationState" : "FAIL",
  "verificationResultDetailedMessage" : "Der vorliegende Beleg weist Fehler auf. Die Fehlerursachen sind nachfolgend angeführt:",
  "verificationResultList" : [ {
    "verificationState" : "FAIL",
    "verificationName" : "Kryptographische Prüfungen",
    "verificationResultList" : [ {
      "verificationState" : "FAIL",
      "verificationName" : "Signatur",
      "verificationResultDetailedMessage" : "Die kryptographische Gültigkeit der Signatur ist nicht gegeben."
    }, {
      "verificationState" : "PASS",
      "verificationResultDetailedMessage" : "ignored"
    } ]
  } ]
}
`

// The leaf failure has only a name; the top-level message is more useful.
const receiptNameOnly = `Machine readable code validation # 0: FAIL
DETAILS:
{
  "verificationState" : "FAIL",
  "verificationResultDetailedMessage" : "Die Registrierkasse war noch nicht in Betrieb genommen.",
  "verificationResultList" : [ {
    "verificationState" : "FAIL",
    "verificationName" : "Statusprüfung der Kasse"
  } ]
}
`

const depStdoutFail = `Machine readable code validation # 0: PASS
DETAILS:
{
  "verificationState" : "PASS",
  "verificationResultDetailedMessage" : "Die Registrierung war erfolgreich."
}
RKSV-DEP-EXPORT-validation # 0: FAIL
{
  "verificationState" : "FAIL",
  "verificationResultList" : [ {
    "verificationState" : "FAIL",
    "verificationName" : "Zusammenfassung der Ergebnisse pro Beleg"
  } ]
}
`

const depResultFile = `{
  "verificationId" : "RKSV_DEP_EXPORT",
  "verificationState" : "FAIL",
  "verificationResultDetailedMessage" : "Am Beginn des exportierten RKSV-DEP befindet sich kein gültiger Startbeleg.",
  "verificationResultList" : [ {
    "verificationState" : "FAIL",
    "verificationResultDetailedMessage" : "Am Beginn des exportierten RKSV-DEP befindet sich kein gültiger Startbeleg."
  } ]
}`

func TestDetermineVerdict(t *testing.T) {
	cases := []struct {
		name, output string
		exit         int
		want         string
	}{
		{"pass", receiptPass, 0, Pass},
		{"fail", receiptFail, 0, Fail},
		{"dep fail after pass", depStdoutFail, 0, Fail},
		{"nonzero exit", receiptPass, 1, Error},
		{"no markers", "Exception in thread main", 0, Unknown},
	}
	for _, c := range cases {
		if got := DetermineVerdict(c.output, c.exit); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestExtractReasons(t *testing.T) {
	depDir := t.TempDir()
	for _, name := range []string{"0000_dep.json", "0001_dep.json"} {
		if err := os.WriteFile(filepath.Join(depDir, name), []byte(depResultFile), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name, output, dir, jobType, verdict string
		want                                []string
	}{
		{"pass summary", receiptPass, "", "receipts", Pass,
			[]string{"Der vorliegende Startbeleg wurde gesetzeskonform erstellt."}},
		{"deepest failure", receiptFail, "", "receipts", Fail,
			[]string{"Die kryptographische Gültigkeit der Signatur ist nicht gegeben."}},
		{"name-only leaf uses parent message", receiptNameOnly, "", "receipts", Fail,
			[]string{"Die Registrierkasse war noch nicht in Betrieb genommen."}},
		{"dep cause from result files, deduplicated", depStdoutFail, depDir, "dep", Fail,
			[]string{"Am Beginn des exportierten RKSV-DEP befindet sich kein gültiger Startbeleg."}},
		{"dep without result files falls back to name", depStdoutFail, t.TempDir(), "dep", Fail,
			[]string{"Zusammenfassung der Ergebnisse pro Beleg"}},
		{"nothing parseable", "garbage", "", "receipts", Unknown, []string{}},
	}
	for _, c := range cases {
		got := ExtractReasons(c.output, c.dir, c.jobType, c.verdict)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
