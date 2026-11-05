package outliner

import (
	"bytes"
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

func TestAttentionBandUsesTheStrongestSignalAndCurrentTheme(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	today, yesterday, tomorrow := util.StartOfDay(now), util.StartOfDay(now.AddDate(0, 0, -1)), util.StartOfDay(now.AddDate(0, 0, 1))
	for _, name := range []string{"dark", "light"} {
		t.Run(name, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: name})
			cases := []struct {
				name string
				task *models.Task
				want string
			}{
				{"overdue beats review and priority", &models.Task{DueDate: &yesterday, Review: true, Priority: 1}, lipgloss.NewStyle().Foreground(theme.ColorError).Render("▐")},
				{"late review beats today's deadline", &models.Task{NextCheckIn: &yesterday, DueDate: &today, Priority: 1}, lipgloss.NewStyle().Foreground(theme.ColorWarning).Render("▐")},
				{"flagged review", &models.Task{Review: true, Priority: 1}, theme.LensAccentDueTodayStyle.Render("▐")},
				{"check-in today", &models.Task{NextCheckIn: &today}, theme.LensAccentDueTodayStyle.Render("▐")},
				{"deadline today beats priority", &models.Task{DueDate: &today, Priority: 1}, theme.LensAccentDueTodayStyle.Render("▐")},
				{"high priority", &models.Task{Priority: 1}, lipgloss.NewStyle().Foreground(theme.ColorPriority1Title).Render("▐")},
				{"ordinary waiting", &models.Task{WorkflowState: models.StateWaiting}, " "},
				{"ordinary doing", &models.Task{WorkflowState: models.StateDoing}, " "},
				{"future dates", &models.Task{NextCheckIn: &tomorrow, DueDate: &tomorrow, Priority: 2}, " "},
				{"deferred check-in", &models.Task{NextCheckIn: &yesterday, DeferredUntil: &tomorrow}, " "},
				{"hidden review", &models.Task{Review: true, NextCheckIn: &yesterday, Hidden: true}, " "},
				{"done", &models.Task{WorkflowState: models.StateDone, DueDate: &yesterday, Review: true, Priority: 1}, " "},
				{"canceled", &models.Task{WorkflowState: models.StateCanceled, DueDate: &yesterday, Priority: 1}, " "},
				{"empty", nil, " "},
			}
			m := &OutlinerModel{}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					got := m.renderTaskCursorColumn(OutlinerRow{Task: tc.task}, cursorPaint{}, now)
					if got != tc.want || lipgloss.Width(got) != 1 {
						t.Fatalf("band = %q (width %d), want %q in one cell", got, lipgloss.Width(got), tc.want)
					}
				})
			}
		})
	}
}

func TestAttentionBandSharesCursorCellAndYieldsToSelectionWithoutMovingTitles(t *testing.T) {
	task := primaryRowsTask("Readable title", models.StateBacklog)
	task.DueDate = timePtr(time.Now().AddDate(0, 0, -2))
	project := primaryRowsProject("alpha", task)
	m := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID}
	for _, width := range []int{60, 100} {
		layout := m.ComputeOutlinerRowLayout([]OutlinerRow{row}, width)
		titleColumn := -1
		for _, selected := range []bool{false, true} {
			m.MultiSelectMode = selected
			if selected {
				m.ToggleTaskIdentity(m.TaskIdentityForRow(row))
			}
			for _, cursor := range []bool{false, true} {
				line := m.RenderOutlinerTaskRow(row, cursor, width, 0, layout)
				plain := ansi.Strip(line)
				want := "▐"
				if selected {
					want = "✓"
				} else if cursor {
					want = "▌"
				}
				if !strings.HasPrefix(plain, want) || lipgloss.Width(line) > width {
					t.Fatalf("width %d selected=%v cursor=%v: %q", width, selected, cursor, line)
				}
				column := lipgloss.Width(plain[:strings.Index(plain, task.Title)])
				if titleColumn >= 0 && column != titleColumn {
					t.Fatalf("title moved from %d to %d", titleColumn, column)
				}
				titleColumn = column
				var mono bytes.Buffer
				writer := colorprofile.Writer{Forward: &mono, Profile: colorprofile.ASCII}
				if _, err := writer.Write([]byte(line)); err != nil {
					t.Fatal(err)
				}
				cursorVisible := strings.HasPrefix(ansi.Strip(mono.String()), "▌")
				if selected {
					cursorVisible = strings.Contains(mono.String(), "\x1b[7m")
				}
				// The deadline's lateness is in words either way: "overdue" past
				// the relative window, "N days ago" inside it.
				monoText := ansi.Strip(mono.String())
				late := strings.Contains(monoText, "overdue") || strings.Contains(monoText, " ago")
				if !late || (cursor && !cursorVisible) {
					t.Fatalf("monochrome lost the deadline or cursor: %q", mono.String())
				}
			}
			if selected {
				m.ToggleTaskIdentity(m.TaskIdentityForRow(row))
			}
		}
	}
}

func TestCursorLeftHalfMatchesAttentionColorOnRight(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	today, yesterday := util.StartOfDay(now), util.StartOfDay(now.AddDate(0, 0, -1))
	for _, name := range []string{"dark", "light"} {
		t.Run(name, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: name})
			cases := []struct {
				name   string
				task   *models.Task
				accent color.Color
			}{
				{"overdue", &models.Task{DueDate: &yesterday}, theme.ColorError},
				{"late review", &models.Task{NextCheckIn: &yesterday}, theme.ColorWarning},
				{"review", &models.Task{Review: true}, theme.LensAccentDueTodayStyle.GetForeground()},
				{"due today", &models.Task{DueDate: &today}, theme.LensAccentDueTodayStyle.GetForeground()},
				{"priority", &models.Task{Priority: 1}, theme.ColorPriority1Title},
				{"quiet", &models.Task{}, nil},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					for _, flash := range []bool{false, true} {
						m := &OutlinerModel{}
						paint := cursorPaint{marked: true, flash: flash}
						cursor := theme.ColorCursorBarBg
						if flash {
							cursor = theme.ColorFlashBg
						}
						style := lipgloss.NewStyle().Foreground(cursor)
						if tc.accent != nil {
							style = style.Foreground(tc.accent).Background(tc.accent)
						}
						got := m.renderTaskCursorColumn(OutlinerRow{Task: tc.task}, paint, now)
						if want := style.Render("▌"); got != want || lipgloss.Width(got) != 1 {
							t.Fatalf("flash=%v: cell=%q, want matching attention halves %q in one column", flash, got, want)
						}
						var mono bytes.Buffer
						writer := colorprofile.Writer{Forward: &mono, Profile: colorprofile.ASCII}
						if _, err := writer.Write([]byte(got)); err != nil {
							t.Fatal(err)
						}
						if ansi.Strip(mono.String()) != "▌" {
							t.Fatalf("monochrome lost the left-half cursor: %q", mono.String())
						}
					}
				})
			}
		})
	}
}

// Every actual fold route must retain the members it hides, including the
// section-only routes and the singleton project header emitted only on folding.
func TestFoldedAttentionSummariesAcrossModes(t *testing.T) {
	cases := []struct {
		name   string
		mode   chrome.OutlinerModeID
		pinned bool
		review bool
	}{
		{"all", chrome.ModeAll, false, false},
		{"pinned", chrome.ModeAll, true, false},
		{"review", chrome.ModeAll, false, true},
		{"today", chrome.ModeToday, false, false},
		{"next", chrome.ModeNext, false, false},
		{"inbox", chrome.ModeInbox, false, false},
		{"backlog", chrome.ModeBacklog, false, false},
		{"due", chrome.ModeDue, false, false},
		{"recent", chrome.ModeRecent, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := tc.mode
			task := primaryRowsTask("urgent", models.StateBacklog)
			task.Pinned, task.Review = tc.pinned, tc.review
			if mode == chrome.ModeInbox {
				task.WorkflowState = models.StateInbox
			}
			task.DueDate = timePtr(time.Now().AddDate(0, 0, -2))
			project := primaryRowsProject("alpha", task)
			m, runtime := primaryRowsOutliner(project)
			locator := []config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: task.ID}}
			runtime.Planning().SetToday(locator)
			if mode == chrome.ModeNext {
				runtime.Planning().SetNext(locator)
			}
			if mode == chrome.ModeAll || mode == chrome.ModeInbox || mode == chrome.ModeBacklog {
				runtime.Planning().SetToday(nil)
			}
			m.SelectOutlinerMode(mode)
			row, found := rowForTaskID(m, task.ID)
			if !found {
				t.Fatal("fixture task is missing")
			}
			for _, key := range row.Group.CollapseKeys() {
				m.Folds.setSection(key, true)
				m.RebuildRows()
				var header *OutlinerRow
				for i := range m.Rows {
					if m.Rows[i].IsCollapsibleHeader() && m.Rows[i].EffectiveCollapseKey() == key {
						header = &m.Rows[i]
					}
				}
				if header == nil {
					t.Fatalf("fold %q produced no header", key)
				}
				for _, cursor := range []bool{false, true} {
					line := m.RenderOutlinerProjectHeaderRow(*header, cursor, 60, 0)
					if header.IsSectionHeader {
						line = m.RenderOutlinerSectionHeaderRow(*header, cursor, 60, 0)
					}
					want := "▐" + theme.IconCollapsed
					if cursor {
						want = "▌" + theme.IconCollapsed
						cell := lipgloss.NewStyle().Foreground(theme.ColorError).Background(theme.ColorError).Render("▌")
						if !strings.HasPrefix(line, cell) {
							t.Fatalf("fold %q cursor covered its overdue accent: %q", key, line)
						}
					}
					if !strings.HasPrefix(ansi.Strip(line), want) || lipgloss.Width(line) > 60 {
						t.Fatalf("fold %q cursor=%v: %q, want prefix %q within 60 columns", key, cursor, line, want)
					}
				}
				if _, found := rowForTaskID(m, task.ID); found {
					t.Fatalf("fold %q left its task visible", key)
				}
				m.Folds.setSection(key, false)
				m.RebuildRows()
			}
		})
	}
}

func TestFoldedAttentionRespectsFiltersAndOtherGroups(t *testing.T) {
	priority := primaryRowsTask("priority", models.StateBacklog)
	priority.Priority, priority.Tags = 1, []string{"keep"}
	urgent := primaryRowsTask("overdue", models.StateBacklog)
	urgent.ParentID = &priority.ID
	urgent.DueDate = timePtr(time.Now().AddDate(0, 0, -2))
	hidden := primaryRowsTask("hidden", models.StateBacklog)
	hidden.DueDate, hidden.Hidden, hidden.Tags = urgent.DueDate, true, []string{"keep"}
	alpha := primaryRowsProject("alpha", priority, urgent, hidden)
	beta := primaryRowsProject("beta", primaryRowsTask("other", models.StateBacklog))
	m, _ := primaryRowsOutliner(alpha, beta)
	m.SetSize(80, 24)
	m.SelectOutlinerMode(chrome.ModeBacklog)
	key := projectGroupCollapseKey(m, alpha.ID)
	betaKey := projectGroupCollapseKey(m, beta.ID)
	m.Folds.setSection(key, true)
	m.Folds.setSection(betaKey, true)
	for _, filtered := range []bool{false, true} {
		if filtered {
			m.Filter.Tags = map[string]bool{"keep": true}
			adoptFilterCarriers(m)
		}
		m.RebuildRows()
		found := false
		for _, row := range m.Rows {
			if row.IsProjectHeader && row.ProjectID == beta.ID {
				if m.renderHeaderAttention(row, cursorPaint{}) != " " {
					t.Fatal("a quiet project inherited another project's band")
				}
				if text := ansi.Strip(strings.Join(m.PeekDocumentBodyForRow(row), "\n")); strings.Contains(text, "Attention:") {
					t.Fatalf("a quiet project inherited another project's Peek summary: %s", text)
				}
			}
			if !row.IsProjectHeader || row.ProjectID != alpha.ID {
				continue
			}
			found = true
			want := chrome.HeroOverdue
			if filtered {
				want = chrome.HeroP1
			}
			if got := strongestAttention(row.attentionTasks, time.Now()).hero; got != want {
				t.Fatalf("filtered=%v summary=%v, want %v", filtered, got, want)
			}
			label := "Overdue deadline"
			if filtered {
				label = "Priority 1"
			}
			if text := ansi.Strip(strings.Join(m.PeekDocumentBodyForRow(row), "\n")); !strings.Contains(text, "Attention: "+label) {
				t.Fatalf("filtered=%v Peek did not explain the band as %q: %s", filtered, label, text)
			}
		}
		if !found {
			t.Fatalf("filtered=%v lost the folded header", filtered)
		}
	}
}

func TestFoldedReviewSummaryPrefersLateCheckIns(t *testing.T) {
	today := primaryRowsTask("today", models.StateBacklog)
	today.NextCheckIn = timePtr(util.StartOfDay(time.Now()))
	late := primaryRowsTask("late", models.StateBacklog)
	late.NextCheckIn = timePtr(util.StartOfDay(time.Now().AddDate(0, 0, -2)))
	flag := primaryRowsTask("flag", models.StateBacklog)
	flag.Review = true
	for _, tasks := range [][]*models.Task{{today, late, flag}, {flag, late, today}} {
		m, _ := primaryRowsOutliner(primaryRowsProject("alpha", tasks...))
		m.SetSize(80, 24)
		m.SelectOutlinerMode(chrome.ModeAll)
		m.Folds.setSection(reviewSectionCollapseKey, true)
		m.RebuildRows()
		found := false
		for index, row := range m.Rows {
			if !row.IsSectionHeader || row.EffectiveCollapseKey() != reviewSectionCollapseKey {
				continue
			}
			found = true
			want := lipgloss.NewStyle().Foreground(theme.ColorWarning).Render("▐")
			if got := m.renderHeaderAttention(row, cursorPaint{}); got != want {
				t.Fatalf("review summary = %q, want late check-in warning %q", got, want)
			}
			if !rowSupportsPeek(row) {
				t.Fatal("a folded section with an attention band cannot open Peek")
			}
			if text := ansi.Strip(strings.Join(m.PeekDocumentBodyForRow(row), "\n")); !strings.Contains(text, "Attention: Late check-in") {
				t.Fatalf("section Peek did not explain its band: %s", text)
			}
			m.Cursor = index
			if !m.OperationCommandContext().Entity.HasFoldedSectionHeaderCursor {
				t.Fatal("the folded section was not offered as a Peek target")
			}
			m.TogglePeek()
			if !m.PeekMode || m.fullscreenPeekTitle() != "Section Details" {
				t.Fatal("the folded section did not open a section Peek")
			}
			m.TogglePeek()
		}
		if !found {
			t.Fatal("folded Review header is missing")
		}
		m.Folds.setSection(reviewSectionCollapseKey, false)
		m.RebuildRows()
		for index, row := range m.Rows {
			if !row.IsSectionHeader || row.EffectiveCollapseKey() != reviewSectionCollapseKey {
				continue
			}
			m.Cursor = index
			if rowSupportsPeek(row) || m.OperationCommandContext().Entity.HasFoldedSectionHeaderCursor {
				t.Fatal("an expanded section offered Peek without a hidden band")
			}
			break
		}
	}
}
