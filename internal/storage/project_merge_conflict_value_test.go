package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// TestMergeConflictsSpellValuesNotAddresses holds a field conflict's text to
// what each version holds. Comments are a slice of pointers; spelling them
// by address made two devices that merged identical documents record
// different conflicts, and so disagree forever about which one was open.
func TestMergeConflictsSpellValuesNotAddresses(t *testing.T) {
	at := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	withComment := func(text string) *ProjectDocument {
		base := reconcileTestBase()
		doc := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
		doc.Tasks[0].Comments = []*models.Comment{{Text: text, CreatedAt: at}}
		return doc
	}
	base := withComment("Branch cut from main.")
	local := withComment("Branch cut at 4f2c (phone).")
	current := withComment("Branch cut from release (desktop).")

	var spelled []string
	for range 2 {
		_, conflicts := mustReconcile(t, base, cloneDoc(local), cloneDoc(current))
		two := mustMerge(t, base, cloneDoc(local), cloneDoc(current))
		for _, conflict := range append(conflicts, two.Conflicts...) {
			if conflict.Field != "comments" {
				continue
			}
			text := conflict.Base + "|" + conflict.Local + "|" + conflict.Current
			if strings.Contains(text, "0x") {
				t.Fatalf("a conflict spells an address: %s", text)
			}
			for _, want := range []string{"Branch cut from main.", "Branch cut at 4f2c (phone).", "Branch cut from release (desktop).", "2026-09-05T12:00:00Z"} {
				if !strings.Contains(text, want) {
					t.Fatalf("a conflict does not name %q: %s", want, text)
				}
			}
			spelled = append(spelled, text)
		}
	}
	if len(spelled) != 4 || spelled[0] != spelled[2] || spelled[1] != spelled[3] {
		t.Fatalf("merging the same versions twice spelled different conflicts: %q", spelled)
	}
}

func cloneDoc(doc *ProjectDocument) *ProjectDocument {
	return mergeTestDoc(doc.ProjectName, cloneDocTasks(doc.Tasks)...)
}
