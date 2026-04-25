package tui

import (
	"strings"
	"testing"
)

func TestTaskForm_DetailsTab_RendersLabels(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToDetailsField(m, 0)
	view := m.View().Content
	for _, label := range []string{"URL:", "Source:"} {
		if !strings.Contains(view, label) {
			t.Errorf("Details tab should contain label %q; got:\n%s", label, view)
		}
	}
}

func TestTaskForm_DetailsTab_URLTyping_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToDetailsField(m, 0)
	before := m.hasChanges
	m = sendFormKey(m, syntheticKey('h'))
	if m.hasChanges == before && !m.hasChanges {
		t.Error("typing in URL field should set hasChanges=true")
	}
}

func TestTaskForm_DetailsTab_SourceTyping_SetsHasChanges(t *testing.T) {
	m := newTestTaskForm()
	m = navigateToDetailsField(m, 1)
	m = sendFormKey(m, syntheticKey('s'))
	if !m.hasChanges {
		t.Error("typing in Source field should set hasChanges=true")
	}
}