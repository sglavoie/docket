package grammar

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCaptureDependencyPreviewNamesEveryReference is the preview half of the
// dependency capture contract: every resolved reference is named in the Depends
// group, in the order the line typed them. Its two siblings compare the typed
// route against the reference picker and so stay in the shell, which is the only
// package that can see the picker.
func TestCaptureDependencyPreviewNamesEveryReference(t *testing.T) {
	home, routed := dependencyWorkspace()
	q := draftedCaptureInput(t, "New work ~2 ~routed:1", home, []*models.Project{home, routed}, false)
	q.UpdatePreview()
	if q.PreviewErr != "" {
		t.Fatalf("preview error: %s", q.PreviewErr)
	}
	groups, ok := q.previewGroups()
	if !ok {
		t.Fatal("the composer painted no preview")
	}
	var depends *previewGroup
	for i, group := range groups {
		if group.label == "Depends" {
			depends = &groups[i]
		}
	}
	if depends == nil {
		t.Fatalf("preview groups = %+v, want a Depends group", groups)
	}
	want := []string{"~Home #2 Ship the API", "~Routed #1 Routed alpha"}
	if !slices.Equal(depends.chunks, want) {
		t.Fatalf("Depends chunks = %v, want %v", depends.chunks, want)
	}
}
