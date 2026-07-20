package tui

import (
	"strings"
	"testing"
)

// TestTutorPlanningNavigationMatchesKeybindings keeps the onboarding and
// concept pipeline tutor topics in sync with the live planning keybindings so
// retired-view language (Focus View, ProjectTree, Due Date Overview) and the
// old "Backlog" label for ;q cannot silently return to maintained help.
func TestTutorPlanningNavigationMatchesKeybindings(t *testing.T) {
	desc := map[string]string{}
	for _, item := range GetKeybindingRegistry().GetGlobalBindings() {
		desc[item.Key] = item.Description
	}

	// The planning shortcuts must advertise the surviving Planner horizons,
	// not the retired views.
	if got := desc[";q"]; !strings.Contains(got, "Planner") || !strings.Contains(got, "Next") {
		t.Errorf(";q keybinding = %q, want Planner/Next", got)
	}
	if got := desc[";w"]; !strings.Contains(got, "Planner") || !strings.Contains(got, "Today") {
		t.Errorf(";w keybinding = %q, want Planner/Today", got)
	}
	if got := desc[";D"]; strings.Contains(got, "Overview") || !strings.Contains(got, "Due Date") {
		t.Errorf(";D keybinding = %q, want All Tasks by Due Date (no 'Overview')", got)
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

	// The concept pipeline topic must name the surviving Planner horizons, the
	// project Outliner, and the by-due-date All Tasks grouping the keybindings
	// advertise.
	concept := topics["The Task Pipeline"]
	for _, want := range []string{
		"Planner — Next",
		"Planner — Today",
		"All Tasks grouped by due date",
		"project outliner",
	} {
		if !strings.Contains(concept, want) {
			t.Errorf("concept pipeline topic missing %q:\n%s", want, concept)
		}
	}

	// The onboarding pipeline topic must label ;q/;w as Planner horizons.
	pipeline := topics["The pipeline"]
	for _, want := range []string{"Planner / Next", "Planner / Today cockpit"} {
		if !strings.Contains(pipeline, want) {
			t.Errorf("onboarding pipeline topic missing %q:\n%s", want, pipeline)
		}
	}
}
