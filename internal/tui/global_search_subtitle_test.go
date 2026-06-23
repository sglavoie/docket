package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestGlobalSearchSubtitleDoesNotMislabelNotesAsTasks(t *testing.T) {
	project, err := models.NewProject("proj-1", "subtitle-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	note1 := &models.Note{Title: "n1"}
	note2 := &models.Note{Title: "n2"}
	allTasks := []models.SearchResult{
		{Note: note1, Project: project},
		{Note: note2, Project: project},
	}
	filtered := []filteredSearchResult{
		{result: allTasks[0]},
		{result: allTasks[1]},
	}

	input := textinput.New()
	input.SetValue("")
	m := &GlobalSearchModel{
		allTasks:      allTasks,
		filtered:      filtered,
		searchInput:   input,
		hideCompleted: true,
		tabs:          TabState{Names: globalSearchTabNames, Active: globalSearchTabNotes},
		listWidth:     30,
		previewWidth:  30,
	}
	m.SetSize(80, 30)

	rendered := m.View().Content

	if strings.Contains(rendered, "2 tasks") {
		t.Fatalf("subtitle should not label %d notes as tasks; rendered:\n%s", len(allTasks), rendered)
	}
}

func TestGlobalSearchSyntaxHintRespectsNarrowWidth(t *testing.T) {
	m := &GlobalSearchModel{}
	m.BaseView.SetSize(44, 18)

	rendered := m.renderSyntaxHint()
	if got := lipgloss.Width(rendered); got > m.Width() {
		t.Fatalf("global search syntax hint exceeded width: got %d > %d for %q", got, m.Width(), rendered)
	}
}

func TestGlobalSearchHelpTextRespectsNarrowWidth(t *testing.T) {
	m := &GlobalSearchModel{
		filtered: []filteredSearchResult{{}, {}, {}},
	}
	m.BaseView.SetSize(44, 18)

	rendered := m.renderHelpText()
	if got := lipgloss.Width(rendered); got > m.Width() {
		t.Fatalf("global search help text exceeded width: got %d > %d for %q", got, m.Width(), rendered)
	}
}
