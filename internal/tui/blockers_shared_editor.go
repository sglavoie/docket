package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const blockerEditorHint = "<!-- Edit the title on the first line below, then description after a blank line. -->"

// blockerBuildEditorCmd creates an exec.Cmd for the configured editor.
func blockerBuildEditorCmd(editor, tmpPath string) (*exec.Cmd, error) {
	resolved, err := external.ResolveEditor(editor)
	if err != nil {
		return nil, err
	}
	return external.EditorCommand(resolved, tmpPath), nil
}

func blockerEditInExternalEditor(s *blockerViewState, ds blockerDataSource, fromEditMode bool) tea.Cmd {
	blocker := ds.getItemBlocker(s.editIndex)
	content := blockerEditorContent(
		fmt.Sprintf("# Blocker: %s", blocker.Title),
		blockerEditorHint,
		blockerTextValue(blocker),
	)
	if fromEditMode {
		content = blockerEditorContent(
			fmt.Sprintf("# Blocker: %s", blocker.Title),
			blockerEditorHint,
			strings.TrimSpace(s.editTextInput.Value()),
		)
	}

	tmpPath, err := writeBlockerTempFile(content)
	if err != nil {
		s.err = err
		return nil
	}

	cmd, editorErr := blockerBuildEditorCmd(s.editor, tmpPath)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return func() tea.Msg { return ErrorMsg{Err: editorErr} }
	}

	editIndex := s.editIndex
	originalTitle := blocker.Title
	originalDesc := blocker.Description

	if fromEditMode {
		s.editMode = false
		s.editTextInput.Reset()
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		title, description, parseErr := readEditedBlockerTempFile(tmpPath)
		if parseErr != nil {
			return ErrorMsg{Err: parseErr}
		}

		if title == originalTitle && description == originalDesc {
			return nil
		}

		edited := ds.getItemBlocker(editIndex)
		edited.Title = title
		edited.Description = description

		project, saveErr := ds.updateBlockerData(editIndex)
		if saveErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", saveErr)}
		}

		return BlockerEditedMsg{Project: project}
	})
}

func blockerAddInExternalEditor(s *blockerViewState, ds blockerDataSource) tea.Cmd {
	project := ds.getAddProject()
	if project == nil {
		s.err = fmt.Errorf("no project available")
		return nil
	}

	content := blockerEditorContent(
		fmt.Sprintf("# New Blocker for %s", project.Name),
		"<!-- Enter the blocker title on the first line, then description after a blank line. -->",
		strings.TrimSpace(s.textInput.Value()),
	)

	tmpPath, err := writeBlockerTempFile(content)
	if err != nil {
		s.err = err
		return nil
	}

	cmd, editorErr := blockerBuildEditorCmd(s.editor, tmpPath)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return func() tea.Msg { return ErrorMsg{Err: editorErr} }
	}

	s.addMode = false
	s.textInput.Reset()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		title, description, parseErr := readEditedBlockerTempFile(tmpPath)
		if parseErr != nil {
			if parseErr.Error() == "blocker title cannot be empty" {
				return nil
			}
			return ErrorMsg{Err: parseErr}
		}

		blocker := models.NewBlocker(title)
		blocker.Description = description
		ds.addBlocker(blocker, project)

		if saveErr := ds.saveBlockerData(blocker, project); saveErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", saveErr)}
		}

		return BlockerAddedMsg{Project: project}
	})
}

func blockerEditorContent(header, hint, body string) string {
	return fmt.Sprintf("%s\n\n%s\n\n%s", header, hint, body)
}

func writeBlockerTempFile(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "tt-blocker-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	return tmpPath, nil
}

func readEditedBlockerTempFile(tmpPath string) (string, string, error) {
	modified, err := os.ReadFile(tmpPath)
	_ = os.Remove(tmpPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read modified file: %w", err)
	}

	return parseBlockerContent(util.ParseEditedCommentContent(string(modified)))
}
