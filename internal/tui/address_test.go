package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gl opens what quick capture puts in a row: a Mail message, a file by its
// URL, or a file by its path, as well as a web address.
func TestGxOpensMailAndFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, "plan.md"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, text, opened, status string }{
		{"mail", "Answer Ann message://%3Cabc@example.com%3E", "message://%3Cabc@example.com%3E", ""},
		{"file url", "Read file:///tmp/a%20b.pdf.", "file:///tmp/a%20b.pdf", ""},
		{"path", "Read " + file + ".", file, ""},
		{"path and line", "Fix " + file + ":12:3", file, ""},
		{"home", "Read ~/plan.md", filepath.Join(home, "plan.md"), ""},
		{"missing path passed over", "See /no/such/file then https://example.com", "https://example.com", ""},
		{"missing path said", "See /no/gone.txt", "", "no file at /no/gone.txt"},
		{"slash in a word is prose", "and/or 3/4", "", "no web address, Mail link or file in this row"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := start(t).wide()
			h.pasteText("Project\n\t" + c.text + " $open")
			h.press("g", "g", "j")
			opened := []string{}
			h.m.openURL = func(url string) error {
				opened = append(opened, url)
				return nil
			}
			steps := len(h.m.undo)
			h.press("g", "x")
			if c.opened == "" {
				if len(opened) != 0 || h.status() != c.status {
					t.Fatalf("opened %v, status %q", opened, h.status())
				}
				return
			}
			if !slices.Equal(opened, []string{c.opened}) || !strings.HasPrefix(h.status(), "opened ") {
				t.Fatalf("opened %v, status %q", opened, h.status())
			}
			if len(h.m.undo) != steps {
				t.Fatal("opening made an undo step")
			}
		})
	}
}

// A Mail link and a file URL are drawn as links, as a web address is; a path
// is not, since knowing it names a file would read the disk while drawing.
func TestMailAndFileLinksDrawn(t *testing.T) {
	for _, w := range []string{"message://%3Cx%3E", "file:///tmp/x", "https://a.b"} {
		if linkOf(w+".") != w {
			t.Fatalf("linkOf(%q) = %q", w+".", linkOf(w+"."))
		}
	}
	if linkOf("/tmp/x") != "" || pathOf("/tmp/x:12.") != "/tmp/x:12" || pathOf("and/or") != "" || pathOf("/") != "" {
		t.Fatal("paths are read as links, or not as paths")
	}
	if !strings.HasPrefix(pathOf("~/a"), "~/") {
		t.Fatal("a home path is not a path")
	}
}
