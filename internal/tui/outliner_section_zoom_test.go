package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/outliner"
)

func sectionHeaderNames(outlinerView *outliner.OutlinerModel) []string {
	names := make([]string, 0)
	for _, row := range outlinerView.Rows {
		if row.IsSectionHeader {
			names = append(names, row.SectionName)
		}
	}
	return names
}

func sectionHeaderIndex(t *testing.T, outlinerView *outliner.OutlinerModel, name string) int {
	t.Helper()
	for index, row := range outlinerView.Rows {
		if row.IsSectionHeader && row.SectionName == name {
			return index
		}
	}
	t.Fatalf("projection has no %q section header, sections are %v", name, sectionHeaderNames(outlinerView))
	return -1
}

func zoomIntoSection(t *testing.T, outlinerView *outliner.OutlinerModel, name string) {
	t.Helper()
	outlinerView.Cursor = sectionHeaderIndex(t, outlinerView, name)
	if !outlinerView.EnterSectionZoom(outlinerView.Rows[outlinerView.Cursor]) {
		t.Fatalf("zoom into %q was refused", name)
	}
}
