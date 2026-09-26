package util

import (
	"runtime"
	"testing"
)

func TestShellTokenize(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		// Basic splitting.
		{"code", []string{"code"}},
		{"nvim -u NORC", []string{"nvim", "-u", "NORC"}},
		{"code --wait", []string{"code", "--wait"}},
		{"", nil},
		{"   ", nil},
		// Single-quoted strings: literal, no escaping.
		{`'/path/with spaces/editor'`, []string{"/path/with spaces/editor"}},
		{`nvim -u '/tmp/my vimrc'`, []string{"nvim", "-u", "/tmp/my vimrc"}},
		// Double-quoted strings: \" and \\ are unescaped.
		{`"/path/with spaces/editor" --wait`, []string{"/path/with spaces/editor", "--wait"}},
		{`nvim -u "/tmp/my vimrc"`, []string{"nvim", "-u", "/tmp/my vimrc"}},
		// Backslash outside quotes.
		{`/path/with\ spaces/editor`, []string{"/path/with spaces/editor"}},
		{`/tmp/O\'Reilly\ report.txt`, []string{"/tmp/O'Reilly report.txt"}},
		{`path\`, []string{`path\`}},
		{"foo\\\nbar", []string{"foobar"}},
		{"cmd \\\n--flag", []string{"cmd", "--flag"}},
		// Mixed quoting styles.
		{`'/opt/my editor/code' --flag "val ue"`, []string{"/opt/my editor/code", "--flag", "val ue"}},
		// Double-quote escape sequences.
		{`"say \"hello\""`, []string{`say "hello"`}},
		{`"back\\slash"`, []string{`back\slash`}},
		{"\"foo\\\nbar\"", []string{"foobar"}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := ShellTokenize(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("ShellTokenize(%q) = %v (len %d), want %v (len %d)", tc.input, got, len(got), tc.want, len(tc.want))
				return
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("ShellTokenize(%q)[%d] = %q, want %q", tc.input, i, v, tc.want[i])
				}
			}
		})
	}
}

func TestNormalizePastedPathInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		// Quoted single paths are unwrapped.
		{"single-quoted path with spaces", `'/path/with spaces/file.txt'`, "/path/with spaces/file.txt"},
		{"double-quoted path with spaces", `"/path/with spaces/file.txt"`, "/path/with spaces/file.txt"},
		{"quoted path trimmed of surrounding whitespace", `  '/path/with spaces/file.txt'  `, "/path/with spaces/file.txt"},
		// Backslash-escaped spaces are unescaped.
		{"escaped spaces", `/path/with\ spaces/file.txt`, "/path/with spaces/file.txt"},
		{"escaped apostrophe and space", `/tmp/O\'Reilly\ report.txt`, "/tmp/O'Reilly report.txt"},
		// Ordinary unquoted paths with spaces are preserved verbatim.
		{"unquoted path with spaces", "/path/with spaces/file.txt", "/path/with spaces/file.txt"},
		// Internal apostrophes are preserved (shell tokenizer would mangle them).
		{"internal apostrophe", "/path/it's a/file.txt", "/path/it's a/file.txt"},
		// Empty and whitespace-only input.
		{"empty input", "", ""},
		{"whitespace-only input", "   ", ""},
		// Plain path with no special handling needed.
		{"plain path", "/path/file.txt", "/path/file.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizePastedPathInput(tc.input); got != tc.want {
				t.Errorf("NormalizePastedPathInput(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizePastedPathInputWindowsBackslashes(t *testing.T) {
	// Unquoted Windows-style backslashes are path separators, not shell escapes.
	// Gate the expectation on runtime.GOOS: on Windows the path is preserved
	// verbatim, while elsewhere the shell tokenizer treats backslashes as escapes.
	const input = `C:\Users\me\file.txt`
	got := NormalizePastedPathInput(input)
	if runtime.GOOS == "windows" {
		if got != input {
			t.Errorf("NormalizePastedPathInput(%q) = %q, want %q (preserved on Windows)", input, got, input)
		}
		return
	}
	if want := "C:Usersmefile.txt"; got != want {
		t.Errorf("NormalizePastedPathInput(%q) = %q, want %q (escaped off Windows)", input, got, want)
	}
}

func TestCountAgreesWithItsNumber(t *testing.T) {
	for n, want := range map[int]string{0: "0 tasks", 1: "1 task", 2: "2 tasks"} {
		if got := Count(n, "task"); got != want {
			t.Errorf("Count(%d) = %q, want %q", n, got, want)
		}
	}
}
