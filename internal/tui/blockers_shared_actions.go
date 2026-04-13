package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

func blockerSaveNew(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	return func() tea.Msg {
		title, _, err := parseBlockerContent(s.textInput.Value())
		if err != nil {
			s.err = err
			return nil
		}

		project := ds.getAddProject()
		if project == nil {
			s.err = fmt.Errorf("no project available")
			return nil
		}

		blocker := models.NewBlocker(title)
		ds.addBlocker(blocker, project)

		if err := ds.saveBlockerData(blocker, project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		s.addMode = false
		s.textInput.Reset()
		s.err = nil
		s.cursor = max(ds.activeCount()-1, 0)

		return BlockerAddedMsg{Project: project}
	}
}

func blockerExecuteDelete(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	return func() tea.Msg {
		visible := ds.getVisibleItems(s.tabs.Active)
		if s.cursor >= len(visible) {
			return nil
		}

		item := visible[s.cursor]
		idx := ds.findItemIndex(item.Blocker)
		if idx < 0 {
			return nil
		}

		project, err := ds.deleteBlockerData(idx)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		visible = ds.getVisibleItems(s.tabs.Active)
		if s.cursor >= len(visible) && s.cursor > 0 {
			s.cursor--
		}

		return BlockerDeletedMsg{Project: project}
	}
}

func blockerStartEdit(s *blockerViewState, ds blockerDataSource, item blockerItem) tea.Cmd {
	idx := ds.findItemIndex(item.Blocker)
	if idx < 0 {
		return nil
	}
	s.editMode = true
	s.editIndex = idx
	s.editTextInput.SetValue(blockerTextValue(item.Blocker))
	s.editTextInput.Focus()
	return textarea.Blink
}

func blockerSaveEdited(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	return func() tea.Msg {
		title, description, err := parseBlockerContent(s.editTextInput.Value())
		if err != nil {
			s.err = err
			return nil
		}

		blocker := ds.getItemBlocker(s.editIndex)
		blocker.Title = title
		blocker.Description = description

		project, saveErr := ds.updateBlockerData(s.editIndex)
		if saveErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", saveErr)}
		}

		s.editMode = false
		s.editTextInput.Reset()
		s.err = nil

		return BlockerEditedMsg{Project: project}
	}
}

func blockerToggleResolve(s *blockerViewState, ds blockerDataSource, item blockerItem) tea.Cmd {
	return func() tea.Msg {
		idx := ds.findItemIndex(item.Blocker)
		if idx < 0 {
			return nil
		}

		blocker := ds.getItemBlocker(idx)
		if blocker.IsResolved() {
			blocker.Unresolve()
		} else {
			blocker.Resolve()
		}

		project, err := ds.updateBlockerData(idx)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		if s.tabs.Active == blockersTabActive || s.tabs.Active == blockersTabResolved {
			visible := ds.getVisibleItems(s.tabs.Active)
			if s.cursor >= len(visible) && s.cursor > 0 {
				s.cursor--
			}
		}

		return BlockerEditedMsg{Project: project}
	}
}

func blockerTextValue(blocker *models.Blocker) string {
	if blocker.Description == "" {
		return blocker.Title
	}
	return blocker.Title + "\n\n" + blocker.Description
}

func parseBlockerContent(content string) (string, string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", tterrors.ErrEmptyTitle
	}

	lines := strings.SplitN(trimmed, "\n", 2)
	title := strings.TrimSpace(lines[0])
	if title == "" {
		return "", "", tterrors.ErrEmptyTitle
	}

	description := ""
	if len(lines) > 1 {
		description = strings.TrimSpace(lines[1])
	}

	return title, description, nil
}
