package outliner

import (
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// peekGuidanceSizes are the two supported terminals every case below is held to.
// The narrow one is where the packer has to spend its budget, so it is what
// makes "the close keys survive" a claim rather than a restatement of the spec.
var peekGuidanceSizes = []struct {
	name          string
	width, height int
}{
	{name: "120x24", width: 120, height: 24},
	{name: "60x20", width: 60, height: 20},
}

// peekGuidanceLongComment wraps to many more rows than the pane's body budget at
// both supported widths, so the window has a top, a genuine middle, and a bottom
// at 120 columns as well as at 60.
func peekGuidanceLongComment() string {
	return strings.Repeat(
		"The vendor contract renewal still needs the finance sign-off before the migration window closes. ",
		12,
	)
}

// peekGuidanceProject carries one stored comment, which is the whole reason its
// header has a Peek document. tasks decides which way the projection ends up
// empty: a project holding none at all, or one whose rows a search hid.
func peekGuidanceProject(id, comment string, tasks bool) *models.Project {
	project := primaryRowsProject(id)
	if tasks {
		project = focusedHeaderProject(id)
	}
	project.Name = "Empty Platform"
	project.Comments = []*models.ProjectComment{peekProjectComment(0, comment)}
	return project
}

// peekGuidanceOutliner scopes to a project whose projection draws no content
// rows and parks the cursor on the focused header it still carries. Peek is left
// closed, because whether one is open is exactly what each case varies.
func peekGuidanceOutliner(t *testing.T, project *models.Project, width, height int, query string) *OutlinerModel {
	t.Helper()
	outliner := scopeTestOutliner(project, primaryRowsProject("other"))
	outliner.SetSize(width, height)
	outliner.PushProjectScope(project.ID)
	outliner.RebuildRows()
	if query != "" {
		outliner.SetSearchQuery(query)
		outliner.RebuildRows()
	}
	if outliner.HasContentRows() {
		t.Fatal("the projection holds content rows, so it is not the empty state this is about")
	}
	if !moveCursorToProjectHeader(outliner, project.ID, 0) {
		t.Fatal("the empty projection built no header for its project")
	}
	if row := outliner.Rows[outliner.Cursor]; !row.IsFocusedProjectHeader {
		t.Fatalf("cursor row = %+v, want the focused project header", row)
	}
	return outliner
}

func peekGuidanceSpec(t *testing.T, m *OutlinerModel, label string) chrome.GuidanceSpec {
	t.Helper()
	spec, ok := m.ContextualGuidance()
	if !ok {
		t.Fatalf("%s: the surface published no guidance at all", label)
	}
	return spec
}

func peekGuidanceEntries(spec chrome.GuidanceSpec) []string {
	entries := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		entries = append(entries, chrome.GuidanceHintText(hint))
	}
	return entries
}

// assertPeekCloseGuidance is the required half of the contract: both keys that
// put the pane down are named, both are reserved ahead of every optional entry,
// and no other entry claims Esc. That last check is what keeps the footer from
// promising an act the first press is not doing, since Esc spends itself on the
// pane before it reaches the query, the lens, or the scope.
func assertPeekCloseGuidance(t *testing.T, m *OutlinerModel, spec chrome.GuidanceSpec, label string) {
	t.Helper()
	closeKey := m.peekOperationKey(operation.OperationCorePeek, peekDisplayKey)
	required := map[string]bool{closeKey + " close Peek": false, "Esc close Peek": false}
	for _, hint := range spec.Hints {
		entry := chrome.GuidanceHintText(hint)
		if _, want := required[entry]; want {
			if !hint.Required {
				t.Errorf("%s: %q is optional, want it reserved ahead of the optional entries", label, entry)
			}
			required[entry] = true
			continue
		}
		if hint.Key == "Esc" {
			t.Errorf("%s: a visible Peek also advertises %q, want Esc to name the pane alone", label, entry)
		}
	}
	for entry, seen := range required {
		if !seen {
			t.Errorf("%s: guidance = %v, want it to name %q", label, peekGuidanceEntries(spec), entry)
		}
	}
}

// assertPeekMovementGuidance holds the offered directions to the ones the window
// can take, read from the same context the four scrolling descriptors consume,
// so an offer and a key can never disagree about whether the pane can move.
func assertPeekMovementGuidance(t *testing.T, m *OutlinerModel, spec chrome.GuidanceSpec, label string) {
	t.Helper()
	peek := m.PeekCommandContext()
	if !peek.Visible {
		t.Fatalf("%s: no pane is on screen, so there is no movement to hold", label)
	}
	movement := map[string]bool{
		m.peekOperationKey(operation.OperationCorePeekHalfPageDown, "Ctrl+D") + " more":  peek.CanPageDown,
		m.peekOperationKey(operation.OperationCorePeekPageDown, "Ctrl+F") + " page down": peek.CanPageDown,
		m.peekOperationKey(operation.OperationCorePeekHalfPageUp, "Ctrl+U") + " back":    peek.CanPageUp,
		m.peekOperationKey(operation.OperationCorePeekPageUp, "Ctrl+B") + " page up":     peek.CanPageUp,
	}
	offered := make(map[string]bool, len(movement))
	for _, entry := range peekGuidanceEntries(spec) {
		if _, ok := movement[entry]; ok {
			offered[entry] = true
		}
	}
	for entry, want := range movement {
		if offered[entry] != want {
			t.Errorf("%s: %q offered = %v, want %v for context %+v (guidance %v)",
				label, entry, offered[entry], want, peek, peekGuidanceEntries(spec))
		}
	}
}

// assertPeekFooterRenders draws the guidance the way the shell draws it and
// holds the three things a reader can see: every line fits the terminal, the
// required entries survived the packing, and the optional entries that survived
// are the priority-ordered prefix the packer promises rather than an arbitrary
// subset. absent names text the frame must not carry at all.
func assertPeekFooterRenders(t *testing.T, spec chrome.GuidanceSpec, width int, absent []string, label string) {
	t.Helper()
	rendered := ansi.Strip(chrome.RenderGuidanceText(spec, width))
	lines := strings.Split(rendered, "\n")
	if rendered == "" {
		t.Fatalf("%s: the footer rendered nothing", label)
	}
	budget := 1
	if spec.Rows >= 2 {
		budget = 2
	}
	if len(lines) > budget {
		t.Errorf("%s: the footer drew %d rows, want at most %d", label, len(lines), budget)
	}
	for index, line := range lines {
		if got := len([]rune(line)); got > width {
			t.Errorf("%s: footer row %d is %d columns, want <= %d: %q", label, index, got, width, line)
		}
	}
	for _, text := range absent {
		if strings.Contains(rendered, text) {
			t.Errorf("%s: the footer reads %q, want it absent:\n%s", label, text, rendered)
		}
	}

	optional := make([]chrome.GuidanceHint, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		if hint.Required {
			if !strings.Contains(rendered, chrome.GuidanceHintText(hint)) {
				t.Errorf("%s: required %q did not survive the %d-column footer:\n%s",
					label, chrome.GuidanceHintText(hint), width, rendered)
			}
			continue
		}
		optional = append(optional, hint)
	}
	sort.SliceStable(optional, func(i, j int) bool { return optional[i].Priority < optional[j].Priority })
	dropped := ""
	for _, hint := range optional {
		entry := chrome.GuidanceHintText(hint)
		if !strings.Contains(rendered, entry) {
			if dropped == "" {
				dropped = entry
			}
			continue
		}
		if dropped != "" {
			t.Errorf("%s: the footer kept %q while dropping the earlier %q, want the priority prefix:\n%s",
				label, entry, dropped, rendered)
		}
	}
}

// peekGuidanceMovementCount is how many of the four scrolling entries the
// guidance is currently offering.
func peekGuidanceMovementCount(m *OutlinerModel, spec chrome.GuidanceSpec) int {
	movement := map[string]bool{
		m.peekOperationKey(operation.OperationCorePeekHalfPageDown, "Ctrl+D") + " more":  true,
		m.peekOperationKey(operation.OperationCorePeekPageDown, "Ctrl+F") + " page down": true,
		m.peekOperationKey(operation.OperationCorePeekHalfPageUp, "Ctrl+U") + " back":    true,
		m.peekOperationKey(operation.OperationCorePeekPageUp, "Ctrl+B") + " page up":     true,
	}
	count := 0
	for _, entry := range peekGuidanceEntries(spec) {
		if movement[entry] {
			count++
		}
	}
	return count
}

// TestEmptyProjectPeekGuidanceMatchesVisiblePane is the claim the empty state
// used to break: the pane a reader is looking at is the pane the footer is
// about. ContextualGuidance answered emptiness before it ever asked whether a
// Peek was drawn, so a focused project holding no visible rows lost the two keys
// that close its open pane and every movement its comment could take — and,
// under a search, offered an Esc that clears the query while the first press was
// closing the pane and keeping it. Emptiness is now asked of the pane rather
// than of the row count alone, so the same header exposes the same controls
// whether or not tasks are listed beneath it.
func TestEmptyProjectPeekGuidanceMatchesVisiblePane(t *testing.T) {
	// The empty state's own recovery hints. None of them may appear while a pane
	// is open, because none of them is what the next press does.
	emptyStateRecovery := []string{"a capture", "Esc clear search", "fx clear filters", "Esc return"}

	t.Run("empty_long", func(t *testing.T) {
		for _, size := range peekGuidanceSizes {
			t.Run(size.name, func(t *testing.T) {
				outliner := peekGuidanceOutliner(t,
					peekGuidanceProject("empty", peekGuidanceLongComment(), false), size.width, size.height, "")
				outliner.PeekMode = true

				row, doc, ok := outliner.peekCursorDocument()
				if !ok {
					t.Fatal("the focused header drew no Peek document")
				}
				if doc.MaxOffset() <= inlinePeekHalfPageRows {
					t.Fatalf("the fixture pages to offset %d, so a half page has no middle to land on",
						doc.MaxOffset())
				}

				// The top: forward alone, alongside both close keys.
				spec := peekGuidanceSpec(t, outliner, "top")
				assertPeekCloseGuidance(t, outliner, spec, "top")
				assertPeekMovementGuidance(t, outliner, spec, "top")
				if peek := outliner.PeekCommandContext(); !peek.CanPageDown || peek.CanPageUp {
					t.Fatalf("top context = %+v, want the way forward alone", peek)
				}
				assertPeekFooterRenders(t, spec, size.width, emptyStateRecovery, "top")

				// The middle: every direction the window has, which is all four.
				if !outliner.HalfPagePeek(1) {
					t.Fatal("the pane would not move off the top")
				}
				spec = peekGuidanceSpec(t, outliner, "middle")
				assertPeekCloseGuidance(t, outliner, spec, "middle")
				assertPeekMovementGuidance(t, outliner, spec, "middle")
				if got := peekGuidanceMovementCount(outliner, spec); got != 4 {
					t.Fatalf("the middle offered %d movements, want all four: %v",
						got, peekGuidanceEntries(spec))
				}
				assertPeekFooterRenders(t, spec, size.width, emptyStateRecovery, "middle")

				// The bottom: the way back alone.
				for outliner.PagePeek(1) { //nolint:revive // Reaching the end is the point of the loop.
				}
				if got := outliner.peekOffsetForRow(row, doc); got != doc.MaxOffset() {
					t.Fatalf("the walk settled at offset %d, want the end %d", got, doc.MaxOffset())
				}
				spec = peekGuidanceSpec(t, outliner, "bottom")
				assertPeekCloseGuidance(t, outliner, spec, "bottom")
				assertPeekMovementGuidance(t, outliner, spec, "bottom")
				if peek := outliner.PeekCommandContext(); peek.CanPageDown || !peek.CanPageUp {
					t.Fatalf("bottom context = %+v, want the way back alone", peek)
				}
				assertPeekFooterRenders(t, spec, size.width, emptyStateRecovery, "bottom")
			})
		}
	})

	t.Run("filtered_empty", func(t *testing.T) {
		const query = "zzzznomatchzzzz"
		for _, size := range peekGuidanceSizes {
			t.Run(size.name, func(t *testing.T) {
				outliner := peekGuidanceOutliner(t,
					peekGuidanceProject("filtered", peekGuidanceLongComment(), true), size.width, size.height, query)
				if got := outliner.ClassifyEmptyCause(); got != outlinerEmptySearch {
					t.Fatalf("empty cause = %d, want the search that hid every row", got)
				}
				before := peekGuidanceSpec(t, outliner, "before Peek")
				if !strings.Contains(strings.Join(peekGuidanceEntries(before), " | "), "Esc clear search") {
					t.Fatalf("the search-empty footer = %v, want it to name clearing the search",
						peekGuidanceEntries(before))
				}

				// With the pane open Esc belongs to the pane, and the footer has to say
				// so rather than repeating the query's recovery key.
				outliner.PeekMode = true
				spec := peekGuidanceSpec(t, outliner, "with Peek")
				assertPeekCloseGuidance(t, outliner, spec, "with Peek")
				assertPeekMovementGuidance(t, outliner, spec, "with Peek")
				assertPeekFooterRenders(t, spec, size.width, emptyStateRecovery, "with Peek")

				// And the first press does what the footer just said, spending itself on
				// the pane and leaving the query where it was.
				if !listsurface.RunEscCascade(outliner.EscSteps()) {
					t.Fatal("the Esc cascade spent nothing")
				}
				if outliner.PeekMode {
					t.Fatal("the first Esc left the pane open")
				}
				if got := outliner.SearchQuery(); got != query {
					t.Fatalf("the first Esc left the query as %q, want %q retained", got, query)
				}
				if !outliner.HasSearchProjection() {
					t.Fatal("the first Esc dropped the search projection")
				}

				// Closed, the cause is the footer's subject again.
				after := peekGuidanceSpec(t, outliner, "after Peek")
				entries := strings.Join(peekGuidanceEntries(after), " | ")
				if !strings.Contains(entries, "Esc clear search") {
					t.Errorf("the restored footer = %v, want it to describe clearing the search", entries)
				}
				if strings.Contains(entries, "close Peek") {
					t.Errorf("the restored footer = %v, want it to stop naming a pane that is closed", entries)
				}
				assertPeekFooterRenders(t, after, size.width, []string{"close Peek"}, "after Peek")
			})
		}
	})

	t.Run("empty_short", func(t *testing.T) {
		for _, size := range peekGuidanceSizes {
			t.Run(size.name, func(t *testing.T) {
				outliner := peekGuidanceOutliner(t,
					peekGuidanceProject("short", "Short note.", false), size.width, size.height, "")
				outliner.PeekMode = true

				if _, doc, ok := outliner.peekCursorDocument(); !ok || doc.Paged() {
					t.Fatalf("the fixture pages (ok=%v), so it is not the immovable pane this covers", ok)
				}
				spec := peekGuidanceSpec(t, outliner, "short")
				assertPeekCloseGuidance(t, outliner, spec, "short")
				assertPeekMovementGuidance(t, outliner, spec, "short")
				if got := peekGuidanceMovementCount(outliner, spec); got != 0 {
					t.Fatalf("an immovable pane offered %d movements: %v", got, peekGuidanceEntries(spec))
				}
				assertPeekFooterRenders(t, spec, size.width, emptyStateRecovery, "short")
			})
		}
	})

	t.Run("closed_peek", func(t *testing.T) {
		for _, test := range []struct {
			name     string
			project  *models.Project
			query    string
			cause    outlinerEmptyCause
			required string
		}{
			{
				name:     "no_tasks",
				project:  peekGuidanceProject("empty", peekGuidanceLongComment(), false),
				cause:    outlinerEmptyNoTasks,
				required: "a capture",
			},
			{
				name:     "hidden_by_search",
				project:  peekGuidanceProject("filtered", peekGuidanceLongComment(), true),
				query:    "zzzznomatchzzzz",
				cause:    outlinerEmptySearch,
				required: "Esc clear search",
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				for _, size := range peekGuidanceSizes {
					t.Run(size.name, func(t *testing.T) {
						outliner := peekGuidanceOutliner(t, test.project, size.width, size.height, test.query)
						if outliner.PeekMode {
							t.Fatal("the fixture opened a pane this case is about not having")
						}
						if got := outliner.ClassifyEmptyCause(); got != test.cause {
							t.Fatalf("empty cause = %d, want %d", got, test.cause)
						}

						spec := peekGuidanceSpec(t, outliner, test.name)
						entries := peekGuidanceEntries(spec)
						joined := strings.Join(entries, " | ")
						if strings.Contains(joined, "Peek") {
							t.Errorf("closed-Peek guidance = %v, want no pane named", entries)
						}
						if !strings.Contains(joined, test.required) {
							t.Errorf("closed-Peek guidance = %v, want the causal recovery %q",
								entries, test.required)
						}
						for _, hint := range spec.Hints {
							if hint.Required && chrome.GuidanceHintText(hint) != test.required {
								t.Errorf("required entry = %q, want the causal %q alone",
									chrome.GuidanceHintText(hint), test.required)
							}
						}
						assertPeekFooterRenders(t, spec, size.width, []string{"close Peek"}, test.name)
					})
				}
			})
		}
	})
}
