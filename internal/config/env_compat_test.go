package config

import (
	"os"
	"testing"
)

func TestGetenvWithLegacy(t *testing.T) {
	t.Run("prefers FTMIND_ when both are set", func(t *testing.T) {
		t.Setenv("FTMIND_LANGUAGE", "en-US")
		t.Setenv("FMIND_LANGUAGE", "zh-CN")
		if got := getenvWithLegacy("LANGUAGE"); got != "en-US" {
			t.Fatalf("expected en-US, got %q", got)
		}
	})

	t.Run("falls back to FMIND_ when FTMIND_ is absent", func(t *testing.T) {
		os.Unsetenv("FTMIND_LANGUAGE")
		t.Setenv("FMIND_LANGUAGE", "zh-CN")
		if got := getenvWithLegacy("LANGUAGE"); got != "zh-CN" {
			t.Fatalf("expected zh-CN, got %q", got)
		}
	})

	t.Run("returns empty when neither is set", func(t *testing.T) {
		os.Unsetenv("FTMIND_LANGUAGE")
		os.Unsetenv("FMIND_LANGUAGE")
		if got := getenvWithLegacy("LANGUAGE"); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
}

func TestSyncBrandEnvironmentVariables(t *testing.T) {
	// Clean slate
	os.Unsetenv("FTMIND_LANGUAGE")
	os.Unsetenv("FMIND_LANGUAGE")

	t.Run("copies FTMIND_ to FMIND_ when legacy is unset", func(t *testing.T) {
		t.Setenv("FTMIND_LANGUAGE", "en-US")
		os.Unsetenv("FMIND_LANGUAGE")
		SyncBrandEnvironmentVariables()
		if got := os.Getenv("FMIND_LANGUAGE"); got != "en-US" {
			t.Fatalf("expected FMIND_LANGUAGE=en-US, got %q", got)
		}
	})

	t.Run("does not overwrite existing FMIND_ value", func(t *testing.T) {
		t.Setenv("FTMIND_LANGUAGE", "en-US")
		t.Setenv("FMIND_LANGUAGE", "zh-CN")
		SyncBrandEnvironmentVariables()
		if got := os.Getenv("FMIND_LANGUAGE"); got != "zh-CN" {
			t.Fatalf("expected FMIND_LANGUAGE to stay zh-CN, got %q", got)
		}
	})
}
