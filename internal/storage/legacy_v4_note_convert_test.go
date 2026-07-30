package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConvertLegacyV4NoteProducesMinimalDeterministicFrontmatter(t *testing.T) {
	path := filepath.Join(
		"testdata/v4_migration/workspace/projects/alpha/notes",
		"project-plan.md",
	)
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read note fixture: %v", err)
	}
	legacy, err := DecodeLegacyV4Note("project-plan.md", source)
	if err != nil {
		t.Fatalf("DecodeLegacyV4Note: %v", err)
	}
	if legacy.Title != "Synthetic project plan" ||
		len(legacy.Tags) != 2 ||
		len(legacy.LinkedTasks) != 1 ||
		!legacy.Pinned {
		t.Fatalf("legacy note lost rich fields: %+v", legacy)
	}
	inputCopy := legacy
	inputCopy.Tags = append([]string(nil), legacy.Tags...)
	inputCopy.LinkedTasks = append([]string(nil), legacy.LinkedTasks...)

	first, err := ConvertLegacyV4Note(legacy)
	if err != nil {
		t.Fatalf("ConvertLegacyV4Note: %v", err)
	}
	second, err := ConvertLegacyV4Note(legacy)
	if err != nil {
		t.Fatalf("second ConvertLegacyV4Note: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("repeated note conversion produced different output")
	}
	if !reflect.DeepEqual(legacy, inputCopy) {
		t.Fatal("note conversion mutated its input")
	}
	if first.Note.Body != "Synthetic project-note body retained by the v5 notes tool." {
		t.Fatalf("note body = %q", first.Note.Body)
	}
	for _, required := range []string{
		"title: Synthetic project plan",
		`created: "2026-01-10T09:00:00Z"`,
		`modified: "2026-02-02T09:00:00Z"`,
		"Synthetic project-note body retained by the v5 notes tool.",
	} {
		if !bytes.Contains(first.Data, []byte(required)) {
			t.Fatalf("converted note is missing %q:\n%s", required, first.Data)
		}
	}
	for _, retired := range []string{"tags:", "linked_tasks:", "pinned:"} {
		if bytes.Contains(first.Data, []byte(retired)) {
			t.Fatalf("converted note retained %q:\n%s", retired, first.Data)
		}
	}
	if len(first.Audit) != 3 {
		t.Fatalf("note audit entries = %+v, want tags links and pin", first.Audit)
	}
	withRetiredField := bytes.Replace(
		first.Data,
		[]byte("created:"),
		[]byte("tags:\n    - leaked\ncreated:"),
		1,
	)
	if err := ValidateV5NoteContent("project-plan.md", withRetiredField); err == nil ||
		!strings.Contains(err.Error(), "unexpected frontmatter field") {
		t.Fatalf("retired target field validation error = %v", err)
	}
}

func TestDecodeLegacyV4NoteRejectsCorruptAndIncompleteFrontmatter(t *testing.T) {
	corrupt, err := os.ReadFile("testdata/v4_migration/unsupported/corrupt-note.md")
	if err != nil {
		t.Fatalf("read corrupt note fixture: %v", err)
	}
	if _, err := DecodeLegacyV4Note("corrupt-note.md", corrupt); err == nil ||
		!strings.Contains(err.Error(), "invalid YAML") {
		t.Fatalf("corrupt note error = %v", err)
	}

	incomplete := []byte("---\ntitle: Missing dates\n---\nbody\n")
	if _, err := DecodeLegacyV4Note("incomplete.md", incomplete); err == nil ||
		!strings.Contains(err.Error(), "invalid created timestamp") {
		t.Fatalf("incomplete note error = %v", err)
	}
}
