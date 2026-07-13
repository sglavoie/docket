package util

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateChildName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"plain file", "notes.md", nil},
		{"dotted file", "my.theme.json", nil},
		{"hidden file", ".keep", nil},
		{"uuid", "550e8400-e29b-41d4-a716-446655440000", nil},
		{"empty", "", ErrPathEmpty},
		{"dot", ".", ErrPathTraversal},
		{"dotdot", "..", ErrPathTraversal},
		{"unix absolute", "/etc/passwd", ErrPathAbsolute},
		{"unix separator", "a/b", ErrPathSeparator},
		{"unix traversal", "../secret", ErrPathSeparator},
		{"windows separator", "a\\b", ErrPathSeparator},
		{"windows traversal", "..\\secret", ErrPathSeparator},
		{"windows drive", "C:\\Windows", ErrPathAbsolute},
		{"windows drive lower", "d:data", ErrPathAbsolute},
		{"unc prefix", "\\\\server\\share", ErrPathAbsolute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateChildName(tc.input)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateChildName(%q) = %v, want nil", tc.input, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateChildName(%q) = %v, want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestSafeJoinAccepts(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		rel  string
	}{
		{"single component", "file.md"},
		{"nested", "sub/file.md"},
		{"deeply nested", "a/b/c/file.md"},
		{"windows nested normalized", "a\\b\\file.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeJoin(root, tc.rel)
			if err != nil {
				t.Fatalf("SafeJoin(%q) unexpected error: %v", tc.rel, err)
			}
			if !withinRoot(root, got) {
				t.Fatalf("SafeJoin(%q) = %q escaped root %q", tc.rel, got, root)
			}
		})
	}
}

func TestSafeJoinRejects(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name    string
		rel     string
		wantErr error
	}{
		{"empty", "", ErrPathEmpty},
		{"unix absolute", "/etc/passwd", ErrPathAbsolute},
		{"unix parent escape", "../outside", ErrPathTraversal},
		{"unix deep escape", "a/../../outside", ErrPathTraversal},
		{"dot component", "a/./b", ErrPathTraversal},
		{"windows parent escape", "..\\..\\outside", ErrPathTraversal},
		{"windows absolute", "C:\\Windows\\system32", ErrPathAbsolute},
		{"unc absolute", "\\\\host\\share", ErrPathAbsolute},
		{"leading separator", "/", ErrPathAbsolute},
		{"doubled separator", "a//b", ErrPathSeparator},
		{"embedded windows traversal", "sub\\..\\..\\etc", ErrPathTraversal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SafeJoin(root, tc.rel)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("SafeJoin(%q) = %v, want %v", tc.rel, err, tc.wantErr)
			}
		})
	}
}

func TestSafeJoinEmptyRoot(t *testing.T) {
	if _, err := SafeJoin("", "file.md"); !errors.Is(err, ErrPathEmpty) {
		t.Fatalf("SafeJoin with empty root = %v, want ErrPathEmpty", err)
	}
}

// TestSafeJoinRejectsSymlinkEscape verifies that a reference passing through an
// existing symlink that points outside root is rejected, not just lexical "..".
func TestSafeJoinRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on Windows CI")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	// root/escape -> outside
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := SafeJoin(root, "escape/secret.txt"); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("SafeJoin through escaping symlink = %v, want ErrPathEscape", err)
	}
	// A symlink that stays inside root is fine.
	inside := filepath.Join(root, "real")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatalf("mkdir inside: %v", err)
	}
	if err := os.Symlink(inside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := SafeJoin(root, "link/file.txt"); err != nil {
		t.Fatalf("SafeJoin through contained symlink = %v, want nil", err)
	}
}

func FuzzValidateChildName(f *testing.F) {
	for _, s := range []string{"file.txt", "../x", "a/b", "a\\b", ".", "..", "", "C:\\x", ".keep"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		if err := ValidateChildName(name); err != nil {
			return
		}
		// An accepted name must be a single, separator-free, non-dot component.
		if strings.ContainsAny(name, `/\`) {
			t.Fatalf("accepted name with separator: %q", name)
		}
		if name == "." || name == ".." || name == "" {
			t.Fatalf("accepted dot/empty component: %q", name)
		}
		if name != filepath.Base(name) {
			t.Fatalf("accepted multi-component name: %q", name)
		}
	})
}

func FuzzSafeJoin(f *testing.F) {
	seeds := []string{
		"a/b", "../etc/passwd", "..\\..\\win", "/abs", "C:\\x",
		"a/../b", "a/./b", "sub\\..\\..\\x", "file.md", "a//b", "",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, rel string) {
		root := t.TempDir()
		got, err := SafeJoin(root, rel)
		if err != nil {
			return
		}
		// Invariant: any accepted path stays lexically within root and contains
		// no traversal component after cleaning.
		if !withinRoot(root, got) {
			t.Fatalf("SafeJoin(%q) = %q escaped root %q", rel, got, root)
		}
		relPart, relErr := filepath.Rel(root, got)
		if relErr != nil {
			t.Fatalf("filepath.Rel(%q, %q): %v", root, got, relErr)
		}
		for _, part := range strings.Split(filepath.ToSlash(relPart), "/") {
			if part == ".." {
				t.Fatalf("SafeJoin(%q) = %q has traversal component", rel, got)
			}
		}
	})
}
