package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/models"
)

// TestBreadcrumbLabelTruncationIsUTF8Safe verifies that the breadcrumb label
// truncation does not slice in the middle of a multi-byte rune. The previous
// implementation used byte-position slicing (label[:n]) which produced invalid
// UTF-8 when project names contained non-ASCII characters.
func TestBreadcrumbLabelTruncationIsUTF8Safe(t *testing.T) {
	app := newTestAppModel()
	// Project name with multi-byte runes (each emoji + accented char is >1 byte).
	app.currentProject = &models.Project{
		ID:   "p1",
		Name: "日本語プロジェクトéàü",
	}

	label := app.breadcrumbLabel(ViewProjectTree, 10)
	if !utf8.ValidString(label) {
		t.Fatalf("breadcrumbLabel produced invalid UTF-8: %q", label)
	}
	// Should contain an ellipsis indicating truncation occurred.
	if !strings.Contains(label, "…") {
		t.Errorf("expected truncation ellipsis in label, got %q", label)
	}
}

// TestBuildBreadcrumbTruncationIsUTF8Safe verifies that the full-breadcrumb
// truncation pathway preserves valid UTF-8 when project names contain
// multi-byte runes.
func TestBuildBreadcrumbTruncationIsUTF8Safe(t *testing.T) {
	app := newTestAppModel()
	app.width = 24 // small width forces aggressive truncation (maxLen=20)
	app.currentProject = &models.Project{
		ID:   "p1",
		Name: "日本語プロジェクトéàü",
	}
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree}

	crumb := app.buildBreadcrumb()
	if !utf8.ValidString(crumb) {
		t.Fatalf("buildBreadcrumb produced invalid UTF-8: %q", crumb)
	}
}
