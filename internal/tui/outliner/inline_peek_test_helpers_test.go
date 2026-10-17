package outliner

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// pagedPeekTask is a task whose detail cannot fit one window at any supported
// width: a long note, a long description, dates, attachments, comments, a
// context, and tags. Every scrolling claim below is made against it, so none of
// them is only true of a document that happens to be one row over.
func pagedPeekTask() *models.Task {
	task := &models.Task{
		ID:            "paged",
		ShortID:       1,
		Title:         "Selected",
		WorkflowState: models.StateBacklog,
		ResumeNote:    "stopped at the CSV column mapping and the retry budget",
		Description:   strings.Repeat("Wrapped description content ", 12),
		Context:       "platform-core",
		Attachments: []models.Attachment{
			{Filename: "design-spec.pdf"},
			{Filename: "migration-notes.md"},
		},
		Comments: []*models.Comment{{Text: "Decision"}},
		Tags:     []string{"backend", "security"},
	}
	return task
}

// unicodePeekTask is the same shape spelled in wide graphemes and combining
// marks, which is where a window measured in runes rather than in columns would
// overflow the pane it was clamped to.
func unicodePeekTask() *models.Task {
	return &models.Task{
		ID:            "unicode",
		ShortID:       2,
		Title:         "受け入れ基準の確認",
		WorkflowState: models.StateBacklog,
		ResumeNote:    "途中まで確認済み — 残りは請求書の丸め処理とロケール別の桁区切り",
		Description:   strings.Repeat("行の折り返しを確かめるための日本語の本文。 ", 10),
		Context:       "国際化",
		Attachments:   []models.Attachment{{Filename: "仕様書-final.pdf"}},
		Comments:      []*models.Comment{{Text: "決定事項"}},
		Tags:          []string{"ｉ18ｎ", "équipe"},
	}
}

// pagedPeekOutliner puts that task under a cursor with Peek open, which is the
// only state the scrolling keys answer in.
func pagedPeekOutliner(t *testing.T, task *models.Task, width, height int) *OutlinerModel {
	t.Helper()
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := scopeTestOutliner(project)
	outliner.SetSize(width, height)
	outliner.RebuildRows()
	outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})
	outliner.PeekMode = true
	// A frame is drawn for the same reason production draws one before a key
	// arrives: the dock is on screen from here on, and its window is what the
	// scrolling keys move.
	outliner.View()
	return outliner
}

func cursorPeekDocument(t *testing.T, m *OutlinerModel) inlinePeekDocument {
	t.Helper()
	_, doc, ok := m.peekCursorDocument()
	if !ok {
		t.Fatal("cursor names no Peek document")
	}
	return doc
}

func peekOffset(t *testing.T, m *OutlinerModel) int {
	t.Helper()
	row, doc, ok := m.peekCursorDocument()
	if !ok {
		t.Fatal("cursor names no Peek document")
	}
	return m.peekOffsetForRow(row, doc)
}
