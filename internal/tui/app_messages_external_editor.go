package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// =============================================================================
// External Editor Messages
// =============================================================================

// EditTaskContentMsg launches external editor for task title/description. An
// empty Path opens a fresh buffer; a reopened one reuses the file the user just
// saved so a refused title keeps its description. RejectedTitle is the title the
// capture/edit grammar refused, which is both the baseline the returned title is
// compared against and the marker that this open is a retry. Notice is that
// refusal, rendered into the buffer's instruction comment, and Description is
// the uncommitted body a retry must carry back into the buffer.
type EditTaskContentMsg struct {
	Task          *models.Task
	Project       *models.Project
	Path          string
	RejectedTitle string
	Notice        string
	Description   string
}

// TaskContentEditedMsg returns results from external editor. Path, RejectedTitle
// and Notice ride back so a second refusal can reopen the same buffer and an
// abandoned retry can report the refusal that started it.
type TaskContentEditedMsg struct {
	Task           *models.Task
	Project        *models.Project
	Path           string
	RejectedTitle  string
	Notice         string
	NewTitle       string
	NewDescription string
	Err            error
}

// OpenOutlineEditorMsg dumps one project's task tree to an external editor for
// bulk restructuring. The Outliner emits it; the shell owns the round trip.
// Project names which project to outline: the shell reloads it from an
// application snapshot before dumping, so the cursor's own copy is never what
// reaches the buffer.
type OpenOutlineEditorMsg struct {
	Project *models.Project
}

// OutlineEditedMsg returns one edited outline buffer. Project and Revision are
// the snapshot pair the buffer was dumped from, and the revision travels into
// the apply so a workspace that moved while the editor was open is refused
// instead of merged. ScopeTaskIDs are the tasks the dump actually printed, which
// is what makes absence readable as deletion. Path is kept so a refused buffer
// can be named in the status rather than silently discarded.
type OutlineEditedMsg struct {
	Project      *models.Project
	Revision     string
	ScopeTaskIDs []string
	Path         string
	Buffer       string
	Err          error
}
