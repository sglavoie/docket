package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// This file is the UX clarity refinement's integrated acceptance pass. Every
// session before it changed one occupant of the frame and proved that occupant
// correct on its own; the failures left after that are the ones no single
// session could see, because they live between two changes that are each right.
//
// So nothing here re-asserts a session's own outcome. ux_clarity_baseline_test.go
// owns those and names the session that wrote each one. What is here is the
// arithmetic that spans them — whether the rows one occupant reserves are the
// rows another one draws in, whether a resize that moves all of them at once
// comes back to where it started, and whether text no fixture carries still fits
// the terminals the plan is reviewed at.

// uxClarityIntegratedOccupant is one thing that takes rows from the list: it is
// opened by real key presses and its cost is read off the model afterwards
// rather than predicted, because the point of the pass is to compare what an
// occupant charges against what the frame actually spent.
type uxClarityIntegratedOccupant struct {
	name string
	open []string
}

// The three occupants this plan gave the frame or moved: the Peek dock sized as
// a share of it (session 7), the menu bar row (pre-existing, and the one
// occupant charged in ContentHeight alone), and the expanded query readout
// inside the status strip (session 5). Each is opened from the resting Outliner
// by the key a user presses.
var uxClarityIntegratedOccupants = []uxClarityIntegratedOccupant{
	{name: "peek", open: []string{"b"}},
	{name: "menu-bar", open: []string{"M"}},
	{name: "readout", open: []string{"Q"}},
}

// TestUXClarityIntegratedFrameSpendsEveryRowOnce is the pass's central
// assertion, and it is deliberately not a restatement of any one layer's
// arithmetic. It adds up what the separate authorities each say they drew — the
// header rows the Outliner writes, the list window ComputeOutlinerViewport
// hands the render, the dock peekDockGeometry sized, the strip
// StatusStripHeight reserved, and the footer the shell composed — and requires
// the total to be the terminal.
//
// A row charged twice makes that total short; a row charged to nobody makes it
// long. Both are invisible to a test that reads one occupant, which is why this
// is the pass that owns it: sessions 4, 5, and 7 each added a reservation, and
// the failure they could produce together is one reserving what another already
// did.
//
// The one permitted shortfall is a single row, and only while the dock is open:
// peekDockFooterWrapRows floors the footer charge at one row so the dock cannot
// slide under a footer that wraps for one cursor row and not the next, and a
// footer that did not wrap leaves that row unspent. Anything else is a finding.
func TestUXClarityIntegratedFrameSpendsEveryRowOnce(t *testing.T) {
	for _, size := range uxClarityMatrixSizes {
		for _, frame := range []string{"launch", "combined-context"} {
			for _, opened := range uxClarityIntegratedOpenings() {
				name := fmt.Sprintf("%s/%s/%s", uxClaritySizeName(size), frame, opened.name)
				// combined-context ends in a visual selection, which closes Peek and
				// refuses to reopen it; the header-row case is read without the dock.
				if frame == "combined-context" && strings.Contains(opened.name, "peek") {
					continue
				}
				t.Run(name, func(t *testing.T) {
					driver, view := uxClarityStagedDriver(t, frame, size[0], size[1])
					uxClarityPress(t, driver, uxClarityKeys(opened.keys...)...)

					budget := uxClarityIntegratedBudget(t, driver, view, size[0])
					slack := size[1] - budget.total()
					switch {
					case slack < 0:
						t.Fatalf("the frame drew %d rows into a %d-row terminal: %s", budget.total(), size[1], budget)
					case slack == 0:
					case slack == 1 && budget.dock > 0:
						// The wrap row an open dock reserves and this footer did not need.
					default:
						t.Fatalf("the frame left %d of %d rows charged to nobody: %s", slack, size[1], budget)
					}

					obs := driver.Observe(ObserveOptions{})
					if len(obs.LayoutIssues) > 0 {
						t.Fatalf("%s: %+v\n%s", name, obs.LayoutIssues, obs.PlainScreen)
					}
					// The list is the view, so whatever else the frame is holding it
					// keeps a row to be a list in.
					if budget.list < 1 {
						t.Fatalf("the list window is %d rows: %s", budget.list, budget)
					}
				})
			}
		}
	}
}

// uxClarityIntegratedOpening is one combination of occupants raised together.
type uxClarityIntegratedOpening struct {
	name string
	keys []string
}

// uxClarityIntegratedOpenings is the resting frame, each occupant alone, and all
// three at once. The last entry is the one that matters: two reservations that
// each look right alone are read against the same terminal only when they are
// both standing.
func uxClarityIntegratedOpenings() []uxClarityIntegratedOpening {
	openings := []uxClarityIntegratedOpening{{name: "resting"}}
	var all []string
	for _, occupant := range uxClarityIntegratedOccupants {
		openings = append(openings, uxClarityIntegratedOpening{name: occupant.name, keys: occupant.open})
		all = append(all, occupant.open...)
	}
	return append(openings, uxClarityIntegratedOpening{name: "peek+menu-bar+readout", keys: all})
}

// uxClarityIntegratedFrameBudget is one frame's rows, split by the authority
// that measured each block. Nothing here is derived from another field: that is
// what lets the sum be evidence rather than a rearrangement of one expression.
type uxClarityIntegratedFrameBudget struct {
	header int
	menu   int
	list   int
	dock   int
	strip  int
	footer int
}

func (b uxClarityIntegratedFrameBudget) total() int {
	return b.header + b.menu + b.list + b.dock + b.strip + b.footer
}

func (b uxClarityIntegratedFrameBudget) String() string {
	return fmt.Sprintf("header %d · menu %d · list %d · dock %d · strip %d · footer %d = %d",
		b.header, b.menu, b.list, b.dock, b.strip, b.footer, b.total())
}

// uxClarityIntegratedBudget reads each block from the authority that owns it.
// The list window is asked for the way the render asks for it — the viewport
// function, given the strip the same frame measured — so the number under test
// is the one the rows were drawn into.
//
// The footer is taken from the shell's own ShellFrameMetrics, which is what the
// views underneath reserve against, and then checked against the bar the shell
// actually painted. That check is half the point: a metric that stopped matching
// its own paint would make every sum below agree with itself and with nothing on
// screen.
func uxClarityIntegratedBudget(
	t *testing.T,
	driver *UITestDriver,
	view *outliner.OutlinerModel,
	width int,
) uxClarityIntegratedFrameBudget {
	t.Helper()
	app := driver.App()
	strip := view.StatusStripHeight(width)
	footer := app.shellFrameMetrics().FooterRows()
	if painted := lipgloss.Height(app.renderStatusBar()) - strip; painted != footer {
		t.Fatalf("the shell measured a %d-row footer and painted %d under a %d-row strip",
			footer, painted, strip)
	}
	budget := uxClarityIntegratedFrameBudget{
		header: uxClarityHeaderRows(view, width),
		list:   view.ComputeOutlinerViewport(width, strip).VisibleRows,
		dock:   len(view.PeekDockLines()),
		strip:  strip,
		footer: footer,
	}
	if view.MenuBarVisible() {
		budget.menu = 1
	}
	return budget
}

// TestUXClarityIntegratedResizeReturnsTheFrameItLeft resizes a frame holding
// every occupant this plan touched down to the app's floor and back, and
// requires the screen to come back byte for byte.
//
// A resize is the one act that moves all of them at once, and each of them
// answers to the frame rather than to the cursor: the dock is a share of the
// height, the context row drops whole segments at a width, the readout windows
// its own document, and the footer wraps. That is four independent responses to
// one number, and a response that is not a pure function of it — one that
// remembers a height it saw on the way down, or seats a window it cannot
// re-derive — shows up here and nowhere else.
//
// The round trip is run through the floor and through the widest reviewed size,
// because a clamp that loses information does it at the end it clamps against.
func TestUXClarityIntegratedResizeReturnsTheFrameItLeft(t *testing.T) {
	floor := uxClarityMatrixSizes[0]
	widest := uxClarityMatrixSizes[len(uxClarityMatrixSizes)-1]
	daily := uxClarityDailySize

	for _, via := range [][2]int{floor, widest} {
		t.Run(fmt.Sprintf("via-%s", uxClaritySizeName(via)), func(t *testing.T) {
			driver, view := uxClarityStagedDriver(t, "launch", daily[0], daily[1])
			seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
			// Peek over a document long enough to scroll, so the round trip carries
			// a window offset and not only a height.
			uxClarityPress(t, driver, uxClarityKeys("b", "ctrl+d", "M", "Q")...)
			if !view.PeekDockVisible() {
				t.Fatal("the dock is not open, so the state under test was not reached")
			}

			before := driver.Observe(ObserveOptions{}).PlainScreen
			beforeDock := view.PeekDockRows()

			driver.Resize(via[0], via[1])
			middle := driver.Observe(ObserveOptions{})
			if len(middle.LayoutIssues) > 0 {
				t.Fatalf("at %s: %+v\n%s", uxClaritySizeName(via), middle.LayoutIssues, middle.PlainScreen)
			}
			assertUXScreenFitsItsTerminal(t, middle, via)

			driver.Resize(daily[0], daily[1])
			after := driver.Observe(ObserveOptions{})
			if got := view.PeekDockRows(); got != beforeDock {
				t.Fatalf("the dock is %d rows after the round trip and was %d", got, beforeDock)
			}
			if after.PlainScreen != before {
				t.Fatalf("the frame did not return from %s\nbefore:\n%s\nafter:\n%s",
					uxClaritySizeName(via), before, after.PlainScreen)
			}
		})
	}
}

// uxClarityIntegratedUnicodeTitle is a title no fixture carries: wide
// double-width characters, a combining mark, and an emoji, long enough that
// every reviewed terminal has to decide what to keep. It is typed rather than
// built into the fixture, because the fixture's row population is read by
// committed scenarios that would all be rewritten by adding a row to it.
//
// Its point is the difference between runes and cells. Every fitter in the
// product measures display width, and a title like this is where a fitter that
// counted runes instead would overflow the terminal by exactly the number of
// wide characters it drew.
const uxClarityIntegratedUnicodeTitle = "配送管理システムの再構築 — résumé the ＦＵＬＬＷＩＤＴＨ rollout 🚀 before the audit window closes"

// TestUXClarityIntegratedWideTextFitsEveryReviewedTerminal drives that title
// through the loop it would break: the capture panel while it is being typed,
// the committed row, and Peek over that row, at every reviewed size in both
// palettes.
//
// Peek is the half that matters most. The row is allowed to shorten a title and
// does; Peek is where the whole of it has to be readable, and its body is
// wrapped rather than elided precisely so a title this wide has a counterpart.
// So the assertion is both directions at once: the frame never exceeds its
// terminal in cells, and the title is recoverable in the dock whole.
func TestUXClarityIntegratedWideTextFitsEveryReviewedTerminal(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	for _, variant := range uxClarityReviewVariants {
		for _, size := range uxClarityMatrixSizes {
			t.Run(fmt.Sprintf("%s-%s", variant, uxClaritySizeName(size)), func(t *testing.T) {
				driver, view := uxClarityStagedDriver(t, "launch", size[0], size[1])
				theme.ApplyTheme(theme.ThemeConfig{Name: variant})

				uxClarityPress(t, driver,
					UITestAction{Type: UITestActionKey, Key: "a"},
					UITestAction{Type: UITestActionText, Text: uxClarityIntegratedUnicodeTitle},
				)
				assertUXScreenFitsItsTerminal(t, driver.Observe(ObserveOptions{}), size)

				uxClarityPress(t, driver,
					UITestAction{Type: UITestActionKey, Key: "enter"},
					UITestAction{Type: UITestActionWait},
				)
				assertUXScreenFitsItsTerminal(t, driver.Observe(ObserveOptions{}), size)

				seatCursorOnTaskTitled(t, view, uxClarityIntegratedUnicodeTitle)
				uxClarityPress(t, driver, uxClarityKeys("b")...)
				obs := driver.Observe(ObserveOptions{})
				assertUXScreenFitsItsTerminal(t, obs, size)
				if !view.PeekDockVisible() {
					t.Fatal("b did not open the dock over the wide row")
				}

				// The whole title is in the dock's document, wrapped across as many
				// of its rows as the width needs. Reading it off the joined body is
				// what proves nothing was elided: a fitter that truncated would leave
				// a run of the title that no join can put back.
				if !uxClarityIntegratedDockCarries(t, driver, view, uxClarityIntegratedUnicodeTitle) {
					t.Fatalf("Peek does not carry the whole wide title at %s:\n%s",
						uxClaritySizeName(size), obs.PlainScreen)
				}
			})
		}
	}
}

// uxClarityIntegratedDockCarries reports whether the dock's document holds a
// string, reassembled across the wrap the pane applied. The dock is a window, so
// a long title may be spread over rows and may start below the first one; the
// search pages the document to its end and joins each frame's dock rows before
// looking, which is the same thing a reader scrolling it does.
func uxClarityIntegratedDockCarries(
	t *testing.T,
	driver *UITestDriver,
	view *outliner.OutlinerModel,
	want string,
) bool {
	t.Helper()
	wanted := uxClarityIntegratedSqueeze(want)
	for range 20 {
		var joined strings.Builder
		for _, line := range view.PeekDockLines() {
			joined.WriteString(ansi.Strip(line))
		}
		if strings.Contains(uxClarityIntegratedSqueeze(joined.String()), wanted) {
			return true
		}
		before := view.PeekDockRows()
		uxClarityPress(t, driver, uxClarityKeys("ctrl+d")...)
		if view.PeekDockRows() != before {
			t.Fatalf("paging the dock resized it from %d rows to %d", before, view.PeekDockRows())
		}
	}
	return false
}

// uxClarityIntegratedSqueeze drops the whitespace a wrap introduces and the
// indent the dock hangs its content off, so a string split across two rows
// compares equal to the one that was written.
func uxClarityIntegratedSqueeze(text string) string {
	return strings.Join(strings.Fields(text), "")
}
