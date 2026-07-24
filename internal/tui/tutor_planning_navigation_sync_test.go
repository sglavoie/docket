package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestTutorPlanningNavigationMatchesKeybindings keeps the onboarding and
// concept pipeline tutor topics in sync with the Outliner-as-sole-root model
// and its Ctrl+Esc reset to global All.
func TestTutorPlanningNavigationMatchesKeybindings(t *testing.T) {
	desc := map[string]string{}
	for _, item := range GetKeybindingRegistry().GetGlobalBindings() {
		desc[item.Key] = item.Description
	}

	// Direct semicolon navigation stays retired.
	if got := desc[";q"]; got != "" {
		t.Errorf("removed ;q keybinding is still advertised as %q", got)
	}
	if got := desc[";w"]; got != "" {
		t.Errorf("removed ;w keybinding is still advertised as %q", got)
	}
	if got := desc[";D"]; got != "" {
		t.Errorf("removed ;D keybinding is still advertised as %q", got)
	}

	// The Ctrl+G destination switcher is removed; Ctrl+Esc now resets the
	// global All Outliner.
	if got := desc["ctrl+g"]; got != "" {
		t.Errorf("removed ctrl+g keybinding is still advertised as %q", got)
	}
	if got := desc["ctrl+esc"]; !strings.Contains(strings.ToLower(got), "global all") {
		t.Errorf("Ctrl+Esc keybinding = %q, want global All reset", got)
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
				t.Errorf("tutor topic %q references retired view/concept %q; use the Outliner flows:\n%s", title, term, content)
			}
		}
	}

	// The concept pipeline topic must teach the Outliner-as-sole-root model.
	concept := topics["The Task Pipeline"]
	for _, want := range []string{
		"The Outliner is the sole root view",
		"opens in All on every run",
		"Today section",
		"work modes Next, Backlog, Active, and",
		"Ctrl+Esc resets to a clean global All Outliner",
	} {
		if !strings.Contains(concept, want) {
			t.Errorf("concept pipeline topic missing %q:\n%s", want, concept)
		}
	}

	// The onboarding topic must teach the same Outliner-as-sole-root model.
	pipeline := topics["The pipeline"]
	for _, want := range []string{
		"The Outliner is the sole root view",
		"always opens in All",
		"Tab cycles Next, Backlog, Active, and All",
		"Ctrl+Esc to reset to a",
	} {
		if !strings.Contains(pipeline, want) {
			t.Errorf("onboarding pipeline topic missing %q:\n%s", want, pipeline)
		}
	}
}

// TestOutlinerRootChromeAndTutorHaveNoRetiredGuidance is the completion
// contract for FR15-002. It renders the normal Outliner root status bar and
// scans the in-app tutor and the command-palette "Views" metadata for retired
// root terminology (Home / Dashboard) and the ghost per-project inbox-triage
// shortcut (O), which the live Outliner keymap binds to W.
func TestOutlinerRootChromeAndTutorHaveNoRetiredGuidance(t *testing.T) {
	// 1. Root status bar: build the sole Outliner root and render its status
	//    chrome. The presentation mirror still carries a ViewDashboard sentinel
	//    beneath the root, but it must contribute no visible "Home"/"Dashboard"
	//    breadcrumb segment.
	app := newTestAppModel()
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}

	crumb := app.buildBreadcrumb()
	if strings.Contains(crumb, "Home") {
		t.Errorf("root breadcrumb still shows retired Home segment: %q", crumb)
	}
	if strings.Contains(crumb, "Dashboard") {
		t.Errorf("root breadcrumb still shows retired Dashboard segment: %q", crumb)
	}
	if !strings.Contains(crumb, "Outliner") {
		t.Errorf("root breadcrumb should render the Outliner label, got %q", crumb)
	}
	if statusBar := app.renderStatusBar(); strings.Contains(statusBar, "Home") || strings.Contains(statusBar, "Dashboard") {
		t.Errorf("root status bar exposes retired root chrome:\n%s", statusBar)
	}

	// The retired ViewDashboard sentinel yields no breadcrumb segment, while a
	// real registered view still resolves a label — so removing Home does not
	// blank child or Esc breadcrumbs.
	if label := app.breadcrumbLabel(ViewDashboard, 16); label != "" {
		t.Errorf("ViewDashboard sentinel should have no breadcrumb label, got %q", label)
	}
	if label := app.breadcrumbLabel(ViewCrossProjectOutliner, 32); !strings.Contains(label, "Outliner") {
		t.Errorf("Outliner breadcrumb label = %q, want an Outliner label", label)
	}

	// 2. Tutor guidance: no topic may route per-project inbox triage through the
	//    retired Dashboard or the ghost O shortcut.
	forbiddenTutor := []string{
		"on the Dashboard",
		"dashboard O",
		"dashboard-only",
		"Shortcut O",
		"Open inbox triage for the selected project",
	}
	for _, topic := range tutorTUITopics() {
		for _, term := range forbiddenTutor {
			if strings.Contains(topic.Content, term) {
				t.Errorf("tutor topic %q advertises retired root/ghost triage guidance %q:\n%s",
					topic.Title, term, topic.Content)
			}
		}
	}
	// The tutor must describe per-project triage from the Outliner via the bound
	// W shortcut (or the palette "inbox processor" command).
	inbox := findTutorTUITopic(t, "Inbox Processing")
	if !strings.Contains(inbox.Content, "Press W in the Outliner") {
		t.Errorf("Inbox Processing topic must open per-project triage from the Outliner with W:\n%s", inbox.Content)
	}

	// 3. Command guidance: no view command may advertise the ghost O shortcut for
	//    per-project inbox triage; the live keymap binds it to W.
	for _, cmd := range app.viewCommands() {
		if cmd.Shortcut == "O" {
			t.Errorf("view command %q advertises ghost shortcut O (Outliner keymap binds inbox triage to W)", cmd.Name)
		}
	}
	inboxCmd := findCommand(app.viewCommands(), "inbox processor")
	if inboxCmd == nil {
		t.Fatal("inbox processor command not found")
	}
	if inboxCmd.Shortcut != "W" {
		t.Errorf("inbox processor command Shortcut = %q, want W to match the Outliner keymap", inboxCmd.Shortcut)
	}
}

// TestOutlinerHasNoLegacyProjectDrilldownBindings is the completion contract for
// FR16-001. The retired Ctrl+P and `g o` project-drilldown controls, and the
// "open project outliner" palette command, must be gone: project scope is
// entered only through Ctrl+X and the project-header scope actions, which share
// pushProjectScope and therefore preserve the active mode. The tutor must no
// longer teach Ctrl+P or a separate per-project Outliner.
func TestOutlinerHasNoLegacyProjectDrilldownBindings(t *testing.T) {
	// 1. Keymap + help registry: neither outliner instance binds the retired
	//    ctrl+p drilldown or the `g o` (go) drill-down sequence.
	for name, m := range map[string]*OutlinerModel{
		"single-project": {project: &models.Project{}},
		"cross-project":  {},
	} {
		keys := m.buildKeyMap().HandledKeys()
		for _, retired := range []string{"ctrl+p", "go"} {
			if containsKey(keys, retired) {
				t.Errorf("%s outliner still binds retired drilldown key %q: %v", name, retired, keys)
			}
		}
	}

	// 2. Command registry: no palette command opens a separate project outliner.
	app := newTestAppModel()
	if cmd := findCommand(app.viewCommands(), "open project outliner"); cmd != nil {
		t.Errorf("retired 'open project outliner' command is still registered: %+v", cmd)
	}
	for _, cmd := range app.viewCommands() {
		hay := strings.ToLower(cmd.Name + " " + cmd.Description + " " + strings.Join(cmd.Aliases, " "))
		for _, phrase := range []string{"drill into project", "project outliner", "open project view"} {
			if strings.Contains(hay, phrase) {
				t.Errorf("view command %q still suggests a separate project outliner destination (%q): %+v",
					cmd.Name, phrase, cmd)
			}
		}
	}

	// 3. Tutor content teaches Ctrl+X / project-header scoping, not Ctrl+P or a
	//    separate per-project Outliner.
	for _, topic := range tutorTUITopics() {
		l := strings.ToLower(topic.Content)
		if strings.Contains(l, "ctrl+p") {
			t.Errorf("tutor topic %q still teaches the retired Ctrl+P drilldown:\n%s", topic.Title, topic.Content)
		}
		for _, phrase := range []string{
			"single-project outliner",
			"project's own outliner",
			"highlighted project's outliner",
		} {
			if strings.Contains(l, phrase) {
				t.Errorf("tutor topic %q still describes a separate project outliner (%q):\n%s",
					topic.Title, phrase, topic.Content)
			}
		}
	}
}

// TestTutorContainsNoRetiredSurfaceGuidance scans the full tutor corpus for
// positive instructions that route users to surfaces retired in the
// Outliner-as-sole-root migration: the Dashboard, a project-list surface, and
// separate single-project / cross-project Outliners reached with Ctrl+P.
//
// Explicit negative migration explanations that teach the new model by
// contrast ("There is no Home, Planner, or project destination"; "There is no
// separate per-project outliner") are deliberately allowed: a line is only a
// violation when it names a retired surface without a negation cue. The bare
// CLI command `tt project list` is a real command, not a retired surface, and
// is exempt.
func TestTutorContainsNoRetiredSurfaceGuidance(t *testing.T) {
	// Substrings that only ever name a retired in-app surface. The adjective
	// "cross-project" (cross-project signals/views/triage) is intentionally
	// absent — only retired *surface* names belong here.
	retired := []string{
		"dashboard",
		"project list",
		"single-project outliner",
		"per-project outliner",
		"separate outliner",
		"cross-outliner",
		"cross-project outliner",
		"ctrl+p",
	}
	// Cues that mark a line as an explicit negative migration explanation.
	negationCues := []string{
		"no ", "not ", "never", "retired", "removed", "replaced",
		"no longer", "instead of", "rather than",
	}
	for _, topic := range tutorTopics() {
		for _, rawLine := range strings.Split(topic.Content, "\n") {
			line := strings.ToLower(rawLine)
			// The real `tt project list` CLI command is not a retired surface.
			if strings.Contains(line, "tt project list") {
				continue
			}
			negated := false
			for _, cue := range negationCues {
				if strings.Contains(line, cue) {
					negated = true
					break
				}
			}
			if negated {
				continue
			}
			for _, term := range retired {
				if strings.Contains(line, term) {
					t.Errorf("tutor topic %q positively routes users to retired surface %q; teach the single Outliner root and project scope frames instead:\n%s",
						topic.Title, term, rawLine)
				}
			}
		}
	}
}

// TestTutorContainsNoRetiredPlannerNavigation rejects any tutor topic that
// instructs users to reach the retired Planner surface or drive perspectives
// with the removed numeric mode keys (P0-P9 / "1-9 to switch"). Negative
// mentions that teach the single-root model ("there is no Home, Planner, or
// project destination") are deliberately not matched by these phrases.
func TestTutorContainsNoRetiredPlannerNavigation(t *testing.T) {
	forbidden := []string{
		// Instructional Planner references (retired surface).
		"from Planner",
		"from the Planner",
		"Planner is the single home",
		"Planner Today",
		"Planner Next",
		"Planner Backlog",
		"Manage Next from Planner",
		"Manage Next from the Planner",
		// Obsolete numeric perspective navigation.
		"perspective preset",
		"1-9 to switch",
		"P0-P9",
		"P0–P9",
	}
	for _, topic := range tutorTopics() {
		for _, term := range forbidden {
			if strings.Contains(topic.Content, term) {
				t.Errorf("tutor topic %q references retired Planner/numeric-perspective navigation %q; use the Outliner mode picker (P) and h/l perspectives:\n%s",
					topic.Title, term, topic.Content)
			}
		}
	}
}
