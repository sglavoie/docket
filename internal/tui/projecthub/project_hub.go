package projecthub

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

func NewProjectHubModel(runtime *config.RuntimeState, modes ...route.ProjectHubMode) *ProjectHubModel {
	if runtime == nil {
		runtime = config.EmptyRuntimeState("")
	}
	mode := route.ProjectHubModeAll
	if len(modes) > 0 {
		mode = modes[0]
	}
	m := &ProjectHubModel{runtime: runtime, mode: mode}
	m.InitSearchInput("Search projects", 64, 32)
	m.rebuildProjectSurface("")
	return m
}

func (m *ProjectHubModel) Init() tea.Cmd { return nil }

func (m *ProjectHubModel) Mode() route.ProjectHubMode { return m.mode }

func (m *ProjectHubModel) SetOperationSystem(
	registry *operation.OperationRegistry,
	executor *operation.OperationExecutor,
	dispatch operation.KeyDispatch,
) {
	m.registry = registry
	m.executor = executor
	m.dispatch = dispatch
}

func (m *ProjectHubModel) SetProjectFocus(projectID string) {
	if m.focusedProjectID == projectID {
		return
	}
	m.focusedProjectID = projectID
	m.rebuildProjectSurface(m.currentProjectHubItemID())
}

// SeatCursorOnProject moves the list cursor to a project's row, reporting
// whether the current matches hold one. Creating a project from the Hub lands
// the cursor on the row it made, so the next key acts on that project rather
// than on whichever row the cursor happened to be resting on.
func (m *ProjectHubModel) SeatCursorOnProject(projectID string) bool {
	if m.surface == nil || projectID == "" {
		return false
	}
	if !m.surface.SetCursorToID(listsurface.ListItemID(projectHubItemPrefix + projectID)) {
		return false
	}
	m.preview.invalidate()
	return true
}

func (m *ProjectHubModel) SetTaskCounts(counts map[string]ProjectTaskTally) {
	if len(counts) == 0 {
		m.taskCounts = nil
	} else {
		m.taskCounts = counts
	}
	m.rebuildProjectSurface(m.currentProjectHubItemID())
}

// SetPreviewTaskPainter installs the factory the preview pane paints its task
// rows through. A Hub without one draws the pane's header and no rows, which is
// what a construction outside the app gets.
func (m *ProjectHubModel) SetPreviewTaskPainter(newPainter NewPreviewTaskPainter) {
	m.newPainter = newPainter
	m.preview.invalidate()
}

func (m *ProjectHubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.actions != nil {
		return m.updateProjectHubActions(key)
	}
	// Above the search branch, so the cycle answers from the query input and
	// from browse alike; the action sheet above still owns its whole key stream.
	// The early position is the only thing this branch owns: the press itself
	// resolves through the Hub's strategy like every other browse key, so the
	// registered task-filter operation is what runs.
	if key.String() == "ctrl+t" {
		m.notice = ""
		return m, m.dispatchProjectHubKey("ctrl+t")
	}
	if m.HasActiveSearch() {
		switch key.String() {
		case "esc":
			// Leaving the input keeps the query so the matches stay browsable.
			m.ConfirmSearch()
			return m, nil
		case "enter":
			return m, m.activateProjectHubCurrent()
		case "up", "ctrl+k":
			m.surface.MoveCursor(-1)
			return m, nil
		case "down", "ctrl+j":
			m.surface.MoveCursor(1)
			return m, nil
		case "ctrl+d":
			m.moveProjectHubHalfPage(1)
			return m, nil
		case "ctrl+u":
			m.moveProjectHubHalfPage(-1)
			return m, nil
		case "ctrl+f", "ctrl+b":
			// The preview pane keeps answering while the query is open: it shows the
			// project the cursor is on, and narrowing the list is exactly when reading
			// one project's tasks is worth doing.
			return m, m.dispatchProjectHubKey(key.String())
		}
		if model, cmd, handled := m.HandleSearchMode(key, m, m.applyProjectFilter); handled {
			return model, cmd
		}
	}

	m.notice = ""
	if m.completeProjectHubTopChord(key.String()) {
		return m, nil
	}
	switch key.String() {
	case "j", "down":
		m.surface.MoveCursor(1)
	case "k", "up":
		m.surface.MoveCursor(-1)
	case "g":
		m.pendingTopChord = true
	case "home":
		m.surface.MoveCursorToStart()
	case "G", "end":
		m.surface.MoveCursorToEnd()
	case "ctrl+d":
		m.moveProjectHubHalfPage(1)
	case "ctrl+u":
		m.moveProjectHubHalfPage(-1)
	case "?":
		// Below the search branch on purpose: `?` is a literal character in the
		// query, so help is a browse-only press. The core help descriptor answers
		// only to the Outliner, so raising it through the executor would render the
		// Outliner grammar; the Hub names its own instead.
		return m, help.HelpCmd("Projects", m.GetHelpItems())
	case "u", "ctrl+r":
		// Below the search branch for the same reason as `?`: u is a literal
		// character in the query. The core undo descriptors answer only to the
		// Outliner, so the press asks the shell for the one workspace ledger
		// instead of dispatching, and the shell names what moved.
		return m, status.WorkspaceUndoCmd(key.String() == "ctrl+r")
	case "esc":
		// The shell only routes Esc here while ShouldCaptureEsc holds it, so this
		// is the selection rung of the unwind and never the close.
		m.surface.ClearSelection()
	case "/", "a", "e", "*", "#", "ctrl+n", "v", "ctrl+a", "ctrl+f", "ctrl+b", "<", ">":
		return m, m.dispatchProjectHubKey(key.String())
	case " ", "space":
		return m, m.dispatchProjectHubKey("space")
	case "enter":
		return m, m.activateProjectHubCurrent()
	}
	return m, nil
}

// completeProjectHubTopChord answers the second g of gg and reports the press as
// consumed. Any other key abandons the prefix silently and is then handled
// normally, so a stray g never swallows the key after it.
func (m *ProjectHubModel) completeProjectHubTopChord(key string) bool {
	if !m.pendingTopChord {
		return false
	}
	m.pendingTopChord = false
	if key != "g" {
		return false
	}
	m.surface.MoveCursorToStart()
	return true
}

// dispatchProjectHubKey resolves one browse key through the Hub's own strategy
// and runs whatever operation it reaches. The cursor's project is frozen into
// the context here because every project operation resolves its target from the
// context rather than from a host value.
func (m *ProjectHubModel) dispatchProjectHubKey(key string) tea.Cmd {
	if m.dispatch == nil {
		return nil
	}
	_, cmd := m.dispatch(
		operation.InputStrategyProjectHub,
		[]string{key},
		m.projectHubCommandContext(m.cursorProjectID()),
	)
	return cmd
}

func (m *ProjectHubModel) CycleProjectHubTaskFilter() {
	m.taskFilter = m.taskFilter.next()
	m.rebuildProjectSurface(m.currentProjectHubItemID())
}

// ToggleProjectHubSelection mirrors the Outliner's v: Enter already focuses a
// project and Space already opens actions, so selection needs its own key
// rather than the Enter/Space toggle the standalone bulk editor used.
func (m *ProjectHubModel) ToggleProjectHubSelection() {
	if m.surface == nil {
		return
	}
	if _, err := m.surface.ToggleCurrentSelection(); err != nil {
		m.notice = err.Error()
	}
}

func (m *ProjectHubModel) SelectEveryMatchingProject() {
	if m.surface == nil {
		return
	}
	matches := m.surface.Matches()
	ids := make([]listsurface.ListItemID, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.Item.ID)
	}
	if err := m.surface.SetSelectedIDs(ids); err != nil {
		m.notice = err.Error()
	}
}

func (m *ProjectHubModel) selectionCount() int {
	if m.surface == nil {
		return 0
	}
	return len(m.surface.SelectedIDs())
}

func (m *ProjectHubModel) applyProjectFilter() {
	if m.surface != nil {
		m.surface.SetQuery(m.SearchQuery())
	}
}

func (m *ProjectHubModel) moveProjectHubHalfPage(direction int) {
	if m.surface == nil || direction == 0 {
		return
	}
	m.surface.MoveCursor(direction * chrome.HalfPage(m.surface.Viewport().Height))
}

func (m *ProjectHubModel) currentProjectHubItemID() listsurface.ListItemID {
	if m.surface == nil {
		return ""
	}
	item, ok := m.surface.CurrentItem()
	if !ok {
		return ""
	}
	return item.ID
}
