package aiharness

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestResolveConfigDirPrecedenceAndSources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	override := filepath.Join(t.TempDir(), "explicit")
	if err := os.Mkdir(override, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", "relative/inherited-codex-home")

	resolved, err := ResolveConfigDir(Codex, override+string(os.PathSeparator)+".")
	if err != nil {
		t.Fatalf("ResolveConfigDir override: %v", err)
	}
	if resolved.Source != ConfigDirOverride || resolved.Path != filepath.Clean(override) {
		t.Fatalf("override resolution = %+v", resolved)
	}

	resolved, err = ResolveConfigDir(Codex, "")
	if err != nil {
		t.Fatalf("ResolveConfigDir environment: %v", err)
	}
	if resolved.Source != ConfigDirEnvironment || resolved.Path != "relative/inherited-codex-home" {
		t.Fatalf("environment resolution = %+v", resolved)
	}

	t.Setenv("CODEX_HOME", "")
	resolved, err = ResolveConfigDir(Codex, "")
	if err != nil {
		t.Fatalf("ResolveConfigDir default: %v", err)
	}
	if resolved.Source != ConfigDirNativeDefault || resolved.Path != "~/.codex" {
		t.Fatalf("native-default resolution = %+v", resolved)
	}

	if _, err := ResolveConfigDir(Name("gemini"), ""); !errors.Is(err, ErrUnknownHarness) {
		t.Fatalf("ResolveConfigDir(unknown) error = %v, want ErrUnknownHarness", err)
	}
}

func TestNormalizeExplicitConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, "configs", "codex")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := NormalizeExplicitConfigDir("~/configs/unused/../codex")
	if err != nil {
		t.Fatalf("NormalizeExplicitConfigDir: %v", err)
	}
	if got != configDir {
		t.Fatalf("normalized path = %q, want %q", got, configDir)
	}
	if gotHome, err := NormalizeExplicitConfigDir("~"); err != nil || gotHome != home {
		t.Fatalf("NormalizeExplicitConfigDir(~) = (%q, %v), want %q", gotHome, err, home)
	}
}

func TestNormalizeExplicitConfigDirDoesNotExpandEnvironmentVariables(t *testing.T) {
	root := t.TempDir()
	literal := filepath.Join(root, "$AI_HARNESS_CONFIG_TEST")
	if err := os.Mkdir(literal, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_HARNESS_CONFIG_TEST", "expanded-elsewhere")

	got, err := NormalizeExplicitConfigDir(literal)
	if err != nil {
		t.Fatalf("NormalizeExplicitConfigDir literal env marker: %v", err)
	}
	if got != literal {
		t.Fatalf("path was unexpectedly expanded: got %q, want %q", got, literal)
	}
}

func TestNormalizeExplicitConfigDirRejectsInvalidPaths(t *testing.T) {
	if _, err := NormalizeExplicitConfigDir(""); err == nil {
		t.Fatal("empty path should fail")
	}

	t.Run("missing", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing-\x1b[31m")
		_, err := NormalizeExplicitConfigDir(missing)
		assertQuotedConfigDirPathError(t, err, missing, "does not exist")
	})

	t.Run("not a directory", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "file-\x1b[31m")
		if err := os.WriteFile(filePath, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := NormalizeExplicitConfigDir(filePath)
		assertQuotedConfigDirPathError(t, err, filePath, "not a directory")
	})

	t.Run("access failure", func(t *testing.T) {
		loopPath := filepath.Join(t.TempDir(), "loop-\x1b[31m")
		if err := os.Symlink(filepath.Base(loopPath), loopPath); err != nil {
			t.Skipf("create symlink loop: %v", err)
		}
		_, err := NormalizeExplicitConfigDir(loopPath)
		assertQuotedConfigDirPathError(t, err, loopPath, "access AI harness config directory")
	})
}

func assertQuotedConfigDirPathError(t *testing.T, err error, path, category string) {
	t.Helper()
	if err == nil {
		t.Fatalf("NormalizeExplicitConfigDir(%q) unexpectedly succeeded", path)
	}
	got := err.Error()
	if !strings.Contains(got, category) {
		t.Fatalf("error = %q, want category %q", got, category)
	}
	if strings.Contains(got, "\x1b") {
		t.Fatalf("error contains a raw terminal escape: %q", got)
	}
	wantPath := strconv.Quote(filepath.Clean(path))
	if !strings.Contains(got, wantPath) {
		t.Fatalf("error = %q, want quoted normalized path %s", got, wantPath)
	}
}
