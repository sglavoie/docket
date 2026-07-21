package tui

import (
	"strings"
	"testing"
)

// TestTutorPlanningNavigationMatchesKeybindings keeps the onboarding and
// concept pipeline tutor topics in sync with the consolidated destination
// switcher and the Task Workspace scope model.
func TestTutorPlanningNavigationMatchesKeybindings(t *testing.T) {
	desc := map[string]string{}
	for _, item := range GetKeybindingRegistry().GetGlobalBindings() {
		desc[item.Key] = item.Description
	}

	// Direct semicolon navigation is retired; the global registry exposes only
	// the canonical Ctrl+G destination switcher.
	if got := desc[";q"]; got != "" {
		t.Errorf("removed ;q keybinding is still advertised as %q", got)
	}
	if got := desc[";w"]; got != "" {
		t.Errorf("removed ;w keybinding is still advertised as %q", got)
	}
	if got := desc[";D"]; got != "" {
		t.Errorf("removed ;D keybinding is still advertised as %q", got)
	}
	if got := desc["ctrl+g"]; !strings.Contains(got, "destination") {
		t.Errorf("Ctrl+G keybinding = %q, want destination switcher", got)
	}

	// The onboarding + concept pipeline topics teach navigation. They must not
	// route users to the deleted Focus View / ProjectTree / Due Date Overview.
	topics := map[string]string{
		"The pipeline":      findTutorTopicByTitle(t, "The pipeline").Content,
		"The Task Pipeline": findTutorTopicByTitle(t, "The Task Pipeline").Content,
	}
	forbidden := []string{
		"Focus View", "focus view",
		"project tree", "ProjectTree",
		"Due date overview", "Due Date Overview",
		"focus queue",
	}
	for title, content := range topics {
		for _, term := range forbidden {
			if strings.Contains(content, term) {
				t.Errorf("tutor topic %q references retired view/concept %q; use the Planner/Outliner flows:\n%s", title, term, content)
			}
		}
	}

	// The concept pipeline topic must name the surviving Planner horizons and
	// explain the shared Task Workspace scopes.
	concept := topics["The Task Pipeline"]
	for _, want := range []string{
		"Today, Next, and Backlog",
		"Tasks and Project both open Task Workspace",
		"application scope",
		"selected project scope",
	} {
		if !strings.Contains(concept, want) {
			t.Errorf("concept pipeline topic missing %q:\n%s", want, concept)
		}
	}

	// The onboarding topic must enumerate the consolidated destinations and
	// explain that Tasks and Project are scopes, not separate workspaces.
	pipeline := topics["The pipeline"]
	for _, want := range []string{
		"Home, Planner, Tasks, Project, Review",
		"Tasks and Project share Task Workspace",
		"Planner contains Today",
	} {
		if !strings.Contains(pipeline, want) {
			t.Errorf("onboarding pipeline topic missing %q:\n%s", want, pipeline)
		}
	}
}
