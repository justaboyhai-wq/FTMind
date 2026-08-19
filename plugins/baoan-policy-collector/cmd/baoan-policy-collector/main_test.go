package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRejectsMissingCommand(t *testing.T) {
	var out, err bytes.Buffer
	if code := run(nil, &out, &err); code != 2 || !strings.Contains(err.String(), "usage:") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}

func TestRunExportsCanonicalRawDirectory(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "policies"), 0o755); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "raw")
	var out, errOut bytes.Buffer
	code := run([]string{"export-raw", "--data-dir", dataDir, "--output-dir", outputDir}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), `"exported":0`) {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunDaemonValidatesConfiguredCron(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"daemon", "--data-dir", t.TempDir(), "--incremental-cron", "invalid", "--full-cron", "0 3 * * 0"}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "expected exactly 5 fields") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}
func TestRunParsesVerify(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"verify", "--all", "--data-dir", t.TempDir()}, &out, &err); code != 0 || !strings.Contains(out.String(), "verified") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}

func TestRunVerifyRejectsPackageThatCannotBuildCanonicalDocument(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "policies", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := run([]string{"verify", "--all", "--data-dir", dataDir}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "canonical") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}
func TestRunParsesManifest(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"export-manifest", "--run", "r1", "--data-dir", "data"}, &out, &err); code != 0 || !strings.Contains(out.String(), "r1") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}
