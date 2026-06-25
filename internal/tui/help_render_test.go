package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

// TestIntentForItem_PrimaryTriageKeysPromoted verifies that the six canonical
// primary triage actions (n, e, D, T, u, ?) are routed to the "Triage" intent
// bucket only when first-level help originates from one of the inbox
// processor views. Promotion is intentionally source-view-aware: those keys
// describe the same triage vocabulary inside ViewInboxProcessor and
// ViewGlobalInboxProcessor, but mean different things on the Dashboard
// (n=new project, e=edit project, D=delete, T=trash, u=unarchive). Help must
// not advertise unrelated Dashboard actions as Triage just because they share
// a key letter with the triage vocabulary.
func TestIntentForItem_PrimaryTriageKeysPromoted(t *testing.T) {
	cases := []struct {
		key  string
		desc string
		cat  string
	}{
		{"n", "Skip task (move to next without processing)", "Navigation"},
		{"e", "Edit task (title/description)", "Operations"},
		{"D", "Mark done", "Operations"},
		{"T", "Manage tags", "Operations"},
		{"u", "Undo last operation on current task", "Operations"},
		{"?", "Toggle help", "Navigation"},
	}
	inboxSources := []ViewType{ViewInboxProcessor, ViewGlobalInboxProcessor}
	for _, source := range inboxSources {
		for _, tc := range cases {
			got := intentForItem(HelpItem{Key: tc.key, Description: tc.desc, Category: tc.cat}, source)
			if got != "Triage" {
				t.Errorf("intentForItem(%q, cat=%q, source=%v) = %q, want %q",
					tc.key, tc.cat, source, got, "Triage")
			}
		}
	}
}

// TestIntentForItem_CategoryMapping covers the static category-to-intent map
// for items that are not promoted by direct-key rules. This is the canonical
// fallback path that gives every prefix group a sensible intent home.
func TestIntentForItem_CategoryMapping(t *testing.T) {
	cases := []struct {
		key  string
		cat  string
		want string
	}{
		{"ms", "Move (m prefix)", "Route & Schedule"},
		{"dt", "Due Date", "Route & Schedule"},
		{"zt", "Snooze", "Route & Schedule"},
		{",s", "Status", "Resolve"},
		{"!1", "Priority", "Resolve"},
		{",r", "Recurrence", "Resolve"},
		{".dev", "Stage (s prefix)", "Refine"},
		{"cb", "Linked Tasks", "Refine"},
		{"fa", "Folds", "View Controls"},
		{"Ft", "Filters", "View Controls"},
		{"tn", "Task (t prefix)", "Task & Blockers"},
		{"bv", "Blockers (b prefix)", "Task & Blockers"},
		{"ctrl+o", "Task Operations", "Task & Blockers"},
		{"y", "Clipboard", "Other"},
		{"ctrl+n", "Notes", "Other"},
	}
	for _, tc := range cases {
		got := intentForItem(HelpItem{Key: tc.key, Category: tc.cat}, ViewInboxProcessor)
		if got != tc.want {
			t.Errorf("intentForItem(key=%q, cat=%q) = %q, want %q", tc.key, tc.cat, got, tc.want)
		}
	}
}

// TestIntentForItem_CrossViewJumpsDemoted verifies that broad cross-view
// navigation jumps (`;x` shortcuts, the palette `:`, history hops, the global
// inbox toggle, etc.) are routed to the "Jump to View" bucket. Keeping them
// out of the primary intent buckets is what makes per-view help feel
// "current-mode actions only" — these globals still appear, but in their own
// section at the bottom.
func TestIntentForItem_CrossViewJumpsDemoted(t *testing.T) {
	keys := []string{
		";d", ";r", ";a", ";o", ";w",
		":", "|", "#",
		"ctrl+,", "ctrl+/", "ctrl+i", "ctrl+X",
		"alt+o", "alt+i", "alt+l",
		"ctrl+6", "ctrl+esc",
	}
	for _, k := range keys {
		got := intentForItem(HelpItem{Key: k, Category: "Navigation"}, ViewInboxProcessor)
		if got != "Jump to View" {
			t.Errorf("intentForItem(%q) = %q, want %q", k, got, "Jump to View")
		}
	}
}

// TestIntentForItem_NavigateKeys covers the universal navigation/quit keys
// that should always live in the Navigate bucket so users can find esc, the
// scroll keys, and ctrl+\ in a predictable place.
func TestIntentForItem_NavigateKeys(t *testing.T) {
	keys := []string{"j", "k", "G", "g", "ctrl+u", "ctrl+d", "esc", "ctrl+c", `ctrl+\`}
	for _, k := range keys {
		got := intentForItem(HelpItem{Key: k, Category: "Navigation"}, ViewInboxProcessor)
		if got != "Navigate" {
			t.Errorf("intentForItem(%q) = %q, want %q", k, got, "Navigate")
		}
	}
}

// TestBuildIntentCategoriesReflectsHelpIntentOrder verifies the per-view tab
// labels follow the canonical helpIntentOrder, not insertion order, and that
// only buckets with at least one item appear. Tabs must match the rendered
// section headers in renderByIntentContent so users can navigate to the same
// intent label they see.
func TestBuildIntentCategoriesReflectsHelpIntentOrder(t *testing.T) {
	items := []HelpItem{
		{Key: "j", Description: "Down", Category: "Navigation"},                  // Navigate
		{Key: "n", Description: "Skip", Category: "Navigation"},                  // Triage
		{Key: "ms", Description: "Move to Someday", Category: "Move (m prefix)"}, // Route & Schedule
		{Key: ",s", Description: "Start", Category: "Status"},                    // Resolve
		{Key: "tn", Description: "New task", Category: "Task (t prefix)"},        // Task & Blockers
		{Key: ";d", Description: "Open Daily Review", Category: "Navigation"},    // Jump to View
		{Key: "y", Description: "Copy title", Category: "Clipboard"},             // Other
		{Key: "fa", Description: "Fold all", Category: "Folds"},                  // View Controls
		{Key: "cb", Description: "Create blocker", Category: "Linked Tasks"},     // Refine
	}
	m := NewHelpViewModel(items, ViewInboxProcessor, "Inbox Processor")
	got := m.categories

	want := []string{
		"Triage",
		"Route & Schedule",
		"Resolve",
		"Refine",
		"Task & Blockers",
		"View Controls",
		"Navigate",
		"Jump to View",
		"Other",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("first-level help tabs = %v, want %v", got, want)
	}
}

func TestFocusBoardHelpUsesPipelineIntentBuckets(t *testing.T) {
	items := []HelpItem{
		{Key: "a", Description: "Quick-add board item", Category: "Actions"},
		{Key: "ctrl+i", Description: "Open Global Inbox Processor", Category: "Navigation"},
		{Key: "p", Description: "Plan or set priority", Category: "Actions"},
		{Key: ";R", Description: "Open Roadmap", Category: "Navigation"},
		{Key: "x", Description: "Complete or dismiss item", Category: "Actions"},
		{Key: ";r", Description: "Open Weekly Review", Category: "Navigation"},
		{Key: ":", Description: "Command palette", Category: "Navigation"},
	}

	cases := []struct {
		key  string
		want string
	}{
		{"a", "Capture"},
		{"ctrl+i", "Triage"},
		{"p", "Plan"},
		{";R", "Plan"},
		{"x", "Work"},
		{";r", "Review"},
		{":", "Jump to View"},
	}
	for _, tc := range cases {
		got := intentForItem(HelpItem{Key: tc.key, Category: "Actions"}, ViewFocusBoard)
		if got != tc.want {
			t.Errorf("intentForItem(%q, ViewFocusBoard) = %q, want %q", tc.key, got, tc.want)
		}
	}

	m := NewHelpViewModel(items, ViewFocusBoard, "Today")
	wantCategories := []string{"Capture", "Triage", "Plan", "Work", "Review", "Jump to View"}
	if !slices.Equal(m.categories, wantCategories) {
		t.Fatalf("Today first-level help tabs = %v, want %v", m.categories, wantCategories)
	}

	m.SetSize(120, 40)
	out := m.renderByIntentContent()
	lastIdx := -1
	for _, header := range wantCategories {
		idx := strings.Index(out, header)
		if idx < 0 {
			t.Fatalf("Today help missing pipeline header %q in:\n%s", header, out)
		}
		if idx <= lastIdx {
			t.Fatalf("Today help header %q appears at byte %d, expected after previous header at %d:\n%s",
				header, idx, lastIdx, out)
		}
		lastIdx = idx
	}
}

// TestRenderByIntentContent_SectionOrderingAndPlacement renders the help view
// and verifies that the intent-bucketed sections appear in the canonical order
// with the primary triage actions in the leading "Triage" section. This is
// the user-visible contract for "reorganize by task intent rather than raw key
// groups": when the user opens help with `?`, the actions they need most for
// the current mode are clustered first and labeled by intent.
func TestRenderByIntentContent_SectionOrderingAndPlacement(t *testing.T) {
	items := []HelpItem{
		{Key: "n", Description: "Skip", Category: "Navigation"},                  // Triage
		{Key: "D", Description: "Mark done", Category: "Operations"},             // Triage
		{Key: "ms", Description: "Move to Someday", Category: "Move (m prefix)"}, // Route & Schedule
		{Key: ",s", Description: "Start", Category: "Status"},                    // Resolve
		{Key: "tn", Description: "New task", Category: "Task (t prefix)"},        // Task & Blockers
		{Key: ";d", Description: "Open Daily Review", Category: "Navigation"},    // Jump to View
		{Key: "j", Description: "Scroll down", Category: "Navigation"},           // Navigate
	}
	m := NewHelpViewModel(items, ViewInboxProcessor, "Inbox Processor")
	m.SetSize(120, 40)
	out := m.renderByIntentContent()

	// Each expected header must appear in order in the rendered text.
	expectedOrder := []string{
		"Triage",
		"Route & Schedule",
		"Resolve",
		"Task & Blockers",
		"Navigate",
		"Jump to View",
	}
	lastIdx := -1
	for _, header := range expectedOrder {
		idx := strings.Index(out, header)
		if idx < 0 {
			t.Fatalf("rendered first-level help missing intent header %q in:\n%s", header, out)
		}
		if idx <= lastIdx {
			t.Fatalf("intent header %q appears at byte %d, expected after previous header at %d:\n%s",
				header, idx, lastIdx, out)
		}
		lastIdx = idx
	}

	// Triage must come before every other bucket — that is the whole point of
	// the redesign. Spot-check that "Skip" (a Triage item) appears before the
	// "Jump to View" header.
	skipIdx := strings.Index(out, "Skip")
	jumpHeaderIdx := strings.Index(out, "Jump to View")
	if skipIdx < 0 || jumpHeaderIdx < 0 || skipIdx >= jumpHeaderIdx {
		t.Fatalf("expected primary triage action 'Skip' to render before 'Jump to View' section; got skip=%d jump=%d in:\n%s",
			skipIdx, jumpHeaderIdx, out)
	}
}

// TestRenderContent_FirstLevelUsesIntent_ExplorerKeepsCategory locks in the
// split between the two help surfaces:
//   - First-level (per-view) help uses intent buckets.
//   - The keybinding explorer keeps raw-Category layout so it remains a
//     faithful, exhaustive reference.
//
// Without this guard, a future refactor could accidentally route the explorer
// through the intent renderer and lose the "full reference" property
// session 9 (Command Palette Escape Hatch) depends on.
func TestRenderContent_FirstLevelUsesIntent_ExplorerKeepsCategory(t *testing.T) {
	items := []HelpItem{
		{Key: "n", Description: "Skip", Category: "Navigation"},
		{Key: ",s", Description: "Start", Category: "Status"},
	}

	firstLevel := NewHelpViewModel(items, ViewInboxProcessor, "Inbox Processor")
	firstLevel.SetSize(120, 40)
	firstLevelOut := firstLevel.renderContent()
	if !strings.Contains(firstLevelOut, "Triage") {
		t.Errorf("first-level help should render intent buckets, missing 'Triage' header:\n%s", firstLevelOut)
	}
	if !strings.Contains(firstLevelOut, "Resolve") {
		t.Errorf("first-level help should render intent buckets, missing 'Resolve' header:\n%s", firstLevelOut)
	}

	explorer := &HelpViewModel{
		items:         items,
		filteredItems: []filteredHelpItem{{HelpItem: items[0]}, {HelpItem: items[1]}},
		sourceView:    ViewKeybindingExplorer,
		viewName:      "All Views",
		isExplorer:    true,
	}
	explorer.categories = explorer.buildCategories()
	explorer.SetSize(120, 40)
	explorerOut := explorer.renderContent()
	// Explorer should keep raw Category labels.
	if !strings.Contains(explorerOut, "Status") {
		t.Errorf("explorer should keep raw Category labels, missing 'Status' header:\n%s", explorerOut)
	}
	if !strings.Contains(explorerOut, "Navigation") {
		t.Errorf("explorer should keep raw Category labels, missing 'Navigation' header:\n%s", explorerOut)
	}
	if strings.Contains(explorerOut, "Triage") {
		t.Errorf("explorer should NOT use intent buckets; found 'Triage' header in:\n%s", explorerOut)
	}
}

// TestIntentForItem_DashboardDoesNotPromoteTriageKeys is the source-aware
// regression: the Dashboard binds n/e/D/T/u to "New project", "Edit project",
// "Delete project", "View trash", and "Unarchive project". Those are not
// triage actions, so first-level help on the Dashboard must not bucket them
// into "Triage" simply because they share key letters with the inbox triage
// vocabulary. The Dashboard's `O` ("Open inbox triage for project") is the
// one exception: it really does open triage, so it belongs in Triage.
func TestIntentForItem_DashboardDoesNotPromoteTriageKeys(t *testing.T) {
	nonTriage := []HelpItem{
		{Key: "n", Description: "New project", Category: "Actions"},
		{Key: "e", Description: "Edit project", Category: "Actions"},
		{Key: "D", Description: "Delete project", Category: "Actions"},
		{Key: "T", Description: "View trash", Category: "Actions"},
		{Key: "u", Description: "Unarchive project", Category: "Actions"},
	}
	for _, item := range nonTriage {
		got := intentForItem(item, ViewDashboard)
		if got == "Triage" {
			t.Errorf("intentForItem(%q, ViewDashboard) = Triage, want any non-Triage bucket", item.Key)
		}
	}

	openTriage := HelpItem{Key: "O", Description: "Open inbox triage for project", Category: "Actions"}
	if got := intentForItem(openTriage, ViewDashboard); got != "Triage" {
		t.Errorf("intentForItem(%q, ViewDashboard) = %q, want Triage", openTriage.Key, got)
	}
}

// TestDashboardFirstLevelHelpTriageBucket renders the Dashboard's first-level
// help and asserts that the "Triage" intent section contains the `O`
// ("Open inbox triage for project") entry but does NOT contain the Dashboard's
// n/e/D/T/u action descriptions. This is the end-to-end version of the
// source-aware classifier: a user opening help from the Dashboard must see
// triage advertised exactly once — as the action that takes them into triage.
func TestDashboardFirstLevelHelpTriageBucket(t *testing.T) {
	cfg, _ := testutil.NewTestConfigWithProject("Alpha")
	cfg.DataDir = t.TempDir()
	dashboard := NewDashboardModel(cfg, nil)
	items := dashboard.GetHelpItems()

	for _, item := range items {
		bucket := intentForItem(item, ViewDashboard)
		switch item.Key {
		case "n", "e", "D", "T", "u":
			if bucket == "Triage" {
				t.Errorf("Dashboard help item %q (%q) classified as Triage; want non-Triage",
					item.Key, item.Description)
			}
		case "O":
			if bucket != "Triage" {
				t.Errorf("Dashboard help item %q (%q) classified as %q; want Triage",
					item.Key, item.Description, bucket)
			}
		}
	}

	help := NewHelpViewModel(items, ViewDashboard, "Dashboard")
	help.SetSize(120, 40)
	out := help.renderByIntentContent()

	if !strings.Contains(out, "Triage") {
		t.Fatalf("Dashboard first-level help missing Triage section:\n%s", out)
	}
	if !strings.Contains(out, "Open inbox triage for project") {
		t.Errorf("Dashboard Triage section missing 'Open inbox triage for project' entry:\n%s", out)
	}

	triageStart := strings.Index(out, "Triage")
	nextHeader := len(out)
	for _, header := range []string{
		"Route & Schedule", "Resolve", "Refine", "Task & Blockers",
		"View Controls", "Navigate", "Jump to View", "Other",
	} {
		if idx := strings.Index(out[triageStart+len("Triage"):], header); idx >= 0 {
			abs := triageStart + len("Triage") + idx
			if abs < nextHeader {
				nextHeader = abs
			}
		}
	}
	triageSection := out[triageStart:nextHeader]
	for _, forbidden := range []string{
		"New project", "Edit project", "Delete project", "View trash", "Unarchive project",
	} {
		if strings.Contains(triageSection, forbidden) {
			t.Errorf("Dashboard Triage section contains non-triage entry %q:\n%s",
				forbidden, triageSection)
		}
	}
}

// TestHelpFooterAdvertisesExplorerAsFullReference verifies the per-view help
// footer points users at the keybinding explorer (`ctrl+\`) for the deep,
// exhaustive reference. This is the discoverability bridge: first-level help
// trims the binding set; the footer tells the user where the rest lives.
func TestHelpFooterAdvertisesExplorerAsFullReference(t *testing.T) {
	items := []HelpItem{{Key: "n", Description: "Skip", Category: "Navigation"}}

	firstLevel := NewHelpViewModel(items, ViewInboxProcessor, "Inbox Processor")
	firstLevel.SetSize(120, 40)
	out := firstLevel.View().Content
	if !strings.Contains(out, `ctrl+\: full reference`) {
		t.Errorf("first-level help footer should advertise ctrl+\\: full reference; got:\n%s", out)
	}

	explorer := NewKeybindingExplorerModel()
	explorer.SetSize(120, 40)
	explorerOut := explorer.View().Content
	if strings.Contains(explorerOut, `ctrl+\: full reference`) {
		t.Errorf("explorer footer should NOT advertise itself as full reference; got:\n%s", explorerOut)
	}
	if !strings.Contains(explorerOut, "p:") {
		t.Errorf("explorer footer should keep its category/prefix toggle hint 'p:'; got:\n%s", explorerOut)
	}
}
