package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSubmissionID(t *testing.T) {
	const id = "cc2f4a67-8d5c-4495-a1b2-0123456789ab"
	for _, log := range []string{
		"Conducting pre-submission checks...\n  id: " + id + "\n  path: x.dmg\nWaiting for processing to complete.\n",
		"Error: HTTPError(statusCode: nil, error: ... NSErrorFailingURLStringKey=https://appstoreconnect.apple.com/notary/v2/submissions/" + id + ", ...)",
	} {
		if got := submissionID(log); got != id {
			t.Errorf("submissionID(%q) = %q", log, got)
		}
	}
	if got := submissionID("no id here"); got != "" {
		t.Errorf("submissionID = %q, want none", got)
	}
}

// TestNotarizeWaitsAgain: a submit that loses its connection while it
// waits, after notarytool named the submission, is waited for again with
// notarytool wait rather than failing the build.
func TestNotarizeWaitsAgain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script stands in for xcrun")
	}
	const id = "cc2f4a67-8d5c-4495-a1b2-0123456789ab"
	bin := t.TempDir()
	log := filepath.Join(bin, "log")
	count := filepath.Join(bin, "waits")
	script := `#!/bin/sh
echo "$1 $2 $3" >> '` + log + `'
case "$1 $2" in
"notarytool submit")
	echo "  id: ` + id + `" >&2
	echo "Error: The Internet connection appears to be offline." >&2
	exit 1;;
"notarytool wait")
	echo x >> '` + count + `'
	if [ "$(wc -l < '` + count + `')" -lt 2 ]; then echo "Error: offline" >&2; exit 1; fi
	echo '{"id": "` + id + `", "status": "Accepted"}';;
*) exit 1;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "xcrun"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	saved := notarizeRetryDelay
	notarizeRetryDelay = 0
	defer func() { notarizeRetryDelay = saved }()

	if err := submitNotary(&Notarize{KeychainProfile: "p"}, "x.dmg", "x.dmg"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	want := "notarytool submit x.dmg\nnotarytool wait " + id + "\nnotarytool wait " + id + "\n"
	if string(data) != want {
		t.Errorf("xcrun ran\n%s\nwant\n%s", data, want)
	}

	// A verdict (Invalid) is not waited for again.
	os.Remove(log)
	script = strings.Replace(script, `exit 1;;
"notarytool wait")`, `echo '{"id": "`+id+`", "status": "Invalid"}'; exit 1;;
"notarytool wait")`, 1)
	if err := os.WriteFile(filepath.Join(bin, "xcrun"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := submitNotary(&Notarize{KeychainProfile: "p"}, "x.dmg", "x.dmg"); err == nil || !strings.Contains(err.Error(), "Invalid") {
		t.Errorf("error %v, want the Invalid verdict", err)
	}
	if data, _ := os.ReadFile(log); string(data) != "notarytool submit x.dmg\n" {
		t.Errorf("xcrun ran\n%s\nwant the submit alone", data)
	}
}
