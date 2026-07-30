package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// ConflictResolutionModel presents one persisted three-way merge conflict and
// its resolution choices. Explicit project-only bundles can keep local, adopt
// remote, or hand-merge in an external editor; retry-only transaction bundles can
// only adopt remote and then retry the original operation. It renders the
// base/local/remote values for each structured conflict and drives resolution
// through the same storage.ResolveConflict boundary the CLI uses.
type ConflictResolutionModel struct {
	BaseView
	ViewBundle

	bundle      *storage.ConflictBundle
	cursor      int
	statusMsg   string
	statusIsErr bool
}

// ShowConflictResolutionMsg opens the resolution view for a loaded conflict.
type ShowConflictResolutionMsg struct {
	Bundle *storage.ConflictBundle
}

// ConflictResolvedMsg reports the outcome of a resolution so the app shell can
// pop the view and surface a status. NewConflictID is set when a re-merge against
// a newer remote produced a fresh conflict instead of committing.
type ConflictResolvedMsg struct {
	ConflictID    string
	NewConflictID string
	Committed     bool
	Adopted       bool // adopted remote (nothing written)
	RetryRequired bool // aborted transaction was not applied; rerun the operation
}

// conflictEditReturnMsg carries the result of an external hand-merge back into
// the view so it can resolve --use file with the edited path.
type conflictEditReturnMsg struct {
	tmpPath string
	err     error
}

// NewConflictResolutionModel builds the resolution view for a conflict bundle.
func NewConflictResolutionModel(runtime *config.RuntimeState, bundle *storage.ConflictBundle) *ConflictResolutionModel {
	m := &ConflictResolutionModel{
		ViewBundle: ViewBundle{runtime: runtime},
		bundle:     bundle,
	}
	m.SetSize(80, 24)
	return m
}

func (m *ConflictResolutionModel) Init() tea.Cmd { return nil }

func (m *ConflictResolutionModel) conflicts() []storage.MergeConflict {
	if m.bundle == nil {
		return nil
	}
	return m.bundle.Record.Conflicts
}

func (m *ConflictResolutionModel) requiresOperationRetry() bool {
	return m.bundle != nil && m.bundle.Record.OperationRetryRequired()
}

// stores builds the conflict store and commit service for the active workspace,
// mirroring the CLI's conflictWorkspace so resolution commits through the same
// transactional boundary.
func (m *ConflictResolutionModel) stores() (*storage.ConflictStore, *storage.CommitService) {
	dataDir := m.runtime.DataDir()
	return storage.NewConflictStore(config.ConflictsDir(dataDir)),
		storage.NewCommitService(config.TransactionsDir(dataDir), []string{dataDir})
}

func (m *ConflictResolutionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case conflictEditReturnMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("editor failed: %v", msg.err))
			return m, nil
		}
		return m, m.resolve(storage.ResolveUseFile, msg.tmpPath)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.conflicts())-1 {
				m.cursor++
			}
			return m, nil
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "l":
			return m, m.resolve(storage.ResolveUseLocal, "")
		case "r":
			return m, m.resolve(storage.ResolveUseRemote, "")
		case "e":
			return m, m.editExternally()
		}
	}
	return m, nil
}

// resolve performs a resolution and returns a command that reports the outcome.
func (m *ConflictResolutionModel) resolve(resolution storage.ConflictResolution, file string) tea.Cmd {
	if m.bundle == nil {
		return nil
	}
	store, commit := m.stores()
	id := m.bundle.Record.ID
	res, err := storage.ResolveConflict(store, commit, id, resolution, file)
	if err != nil {
		if errors.Is(err, storage.ErrRemoteReconflict) {
			newID := res.NewConflictID
			return func() tea.Msg {
				return ConflictResolvedMsg{ConflictID: id, NewConflictID: newID}
			}
		}
		m.setError(err.Error())
		return nil
	}
	committed := res.Committed
	adopted := resolution == storage.ResolveUseRemote
	retryRequired := res.RetryRequired
	return func() tea.Msg {
		return ConflictResolvedMsg{ConflictID: id, Committed: committed, Adopted: adopted, RetryRequired: retryRequired}
	}
}

// editExternally writes the local document to a temp file, opens it in the
// configured editor, and resolves --use file with the edited result.
func (m *ConflictResolutionModel) editExternally() tea.Cmd {
	if m.requiresOperationRetry() {
		m.setError(storage.ErrConflictRetryRequired.Error())
		return nil
	}
	if m.bundle == nil || m.bundle.Local == nil {
		m.setError("no local document to edit")
		return nil
	}
	tmp, err := os.CreateTemp("", "tt-conflict-*.md")
	if err != nil {
		m.setError(err.Error())
		return nil
	}
	tmpPath := tmp.Name()
	content, renderErr := m.bundle.Local.WriteCurrent()
	if renderErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		m.setError(renderErr.Error())
		return nil
	}
	if _, writeErr := tmp.WriteString(content); writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		m.setError(writeErr.Error())
		return nil
	}
	_ = tmp.Close()

	resolved, err := external.ResolveEditor("")
	if err != nil {
		_ = os.Remove(tmpPath)
		m.setError(err.Error())
		return nil
	}
	cmd := external.EditorCommand(resolved, tmpPath)
	return tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		return conflictEditReturnMsg{tmpPath: tmpPath, err: execErr}
	})
}

func (m *ConflictResolutionModel) setError(msg string) {
	m.statusMsg = msg
	m.statusIsErr = true
}

func (m *ConflictResolutionModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// GetHelpItems returns help items for this view.
func (m *ConflictResolutionModel) GetHelpItems() []HelpItem {
	return helpItemsFromKeyMap(m.buildKeyMap())
}

func (m *ConflictResolutionModel) buildKeyMap() *keymap.KeyMap {
	entries := []keymap.Entry{
		keymap.Bind("j", "Next conflict", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor < len(m.conflicts())-1 {
				m.cursor++
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Previous conflict", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("l", "Keep local", "Resolution", func() (tea.Model, tea.Cmd) {
			return m, m.resolve(storage.ResolveUseLocal, "")
		}),
		keymap.Bind("r", "Adopt remote", "Resolution", func() (tea.Model, tea.Cmd) {
			return m, m.resolve(storage.ResolveUseRemote, "")
		}),
		keymap.Bind("e", "Hand-merge in editor", "Resolution", func() (tea.Model, tea.Cmd) {
			return m, m.editExternally()
		}),
	}
	return &keymap.KeyMap{ViewName: "Conflict Resolution", Entries: entries}
}

func (m *ConflictResolutionModel) View() tea.View {
	var b strings.Builder

	if m.bundle == nil {
		b.WriteString(MutedStyle.Render("No conflict to resolve."))
		return tea.NewView(b.String())
	}
	rec := m.bundle.Record

	subtitle := fmt.Sprintf("%s · %d conflict(s)", rec.ProjectName, len(rec.Conflicts))
	b.WriteString(renderFullscreenTitle("Resolve Conflict", subtitle, m.AccentColor()))
	b.WriteString("\n\n")
	if rec.OperationRetryRequired() {
		b.WriteString(PreviewWarningStyle.Render("This transaction was aborted. Adopt remote to clear it, then retry the original operation; local or hand-merged project-only resolution is disabled."))
		b.WriteString("\n\n")
	}

	if m.statusMsg != "" {
		style := DoneStyle
		if m.statusIsErr {
			style = ErrorIconStyle
		}
		b.WriteString(style.Render(m.statusMsg))
		b.WriteString("\n\n")
	}

	if len(rec.Conflicts) == 0 {
		b.WriteString(MutedStyle.Render("No structured conflicts recorded."))
		b.WriteString("\n")
	}

	for i, c := range rec.Conflicts {
		marker := "  "
		label := string(c.Kind)
		if c.Field != "" {
			label += " · " + c.Field
		}
		if c.TaskTitle != "" {
			label += " — " + c.TaskTitle
		}
		if i == m.cursor {
			marker = IconCursorRight + " "
			b.WriteString(SelectedItemTextStyle.Render(marker + label))
		} else {
			b.WriteString(marker + label)
		}
		b.WriteString("\n")

		if i == m.cursor {
			if c.Base != "" || c.Local != "" || c.Current != "" {
				b.WriteString(MutedStyle.Render("      base:   " + c.Base))
				b.WriteString("\n")
				b.WriteString(MutedStyle.Render("      local:  " + c.Local))
				b.WriteString("\n")
				b.WriteString(MutedStyle.Render("      remote: " + c.Current))
				b.WriteString("\n")
			}
			if c.Detail != "" {
				b.WriteString(MutedStyle.Render("      " + c.Detail))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	helpText := "l: keep local · r: adopt remote · e: hand-merge · j/k: nav · Esc: close"
	if rec.OperationRetryRequired() {
		helpText = "r: clear and retry operation · j/k: nav · Esc: close"
	}
	help := HelpStyle.Render(helpText)
	b.WriteString(renderStatusBarFooter(help, "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}
