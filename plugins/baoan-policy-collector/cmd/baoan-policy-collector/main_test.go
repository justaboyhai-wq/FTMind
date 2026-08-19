package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsMissingCommand(t *testing.T) {
	var out, err bytes.Buffer
	if code := run(nil, &out, &err); code != 2 || !strings.Contains(err.String(), "usage:") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}
func TestRunParsesVerify(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"verify", "--all", "--data-dir", t.TempDir()}, &out, &err); code != 0 || !strings.Contains(out.String(), "verification") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}
func TestRunParsesManifest(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"export-manifest", "--run", "r1", "--data-dir", "data"}, &out, &err); code != 0 || !strings.Contains(out.String(), "r1") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
	}
}
