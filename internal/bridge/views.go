package bridge

import (
	"errors"
	"io/fs"
	"os"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// The views below are the bridge's own wire shapes. They copy what a client
// reads out of the domain types rather than serializing those types, so a
// field added to models.Task for the desktop never changes the contract by
// accident.

// locator is a qualified task identity on the wire.
type locator struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
}

func (l locator) validate(field string) *Error {
	if l.ProjectID == "" || l.TaskID == "" {
		return newError(CodeInvalidRequest, "%s needs both project_id and task_id", field)
	}
	return nil
}

func (l locator) app() app.TaskLocator {
	return app.TaskLocator{ProjectID: l.ProjectID, TaskID: l.TaskID}
}

// taskView is one task as a client reads it.
type taskView struct {
	ProjectID      string      `json:"project_id"`
	TaskID         string      `json:"task_id"`
	ShortID        int         `json:"short_id"`
	Title          string      `json:"title"`
	Description    string      `json:"description,omitempty"`
	State          string      `json:"state"`
	Priority       int         `json:"priority"`
	ParentID       string      `json:"parent_id,omitempty"`
	Tags           []string    `json:"tags"`
	Context        string      `json:"context,omitempty"`
	Pinned         bool        `json:"pinned"`
	Review         bool        `json:"review"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	StateChangedAt time.Time   `json:"state_changed_at"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
	DueDate        *time.Time  `json:"due_date,omitempty"`
	DeferredUntil  *time.Time  `json:"deferred_until,omitempty"`
	ResumeNote     string      `json:"resume_note,omitempty"`
	NextCheckIn    *time.Time  `json:"next_check_in,omitempty"`
	Checklist      []checkItem `json:"checklist"`
	Comments       int         `json:"comment_count"`
	Attachments    int         `json:"attachment_count"`
	Dependencies   int         `json:"dependency_count"`
}

// checkItem is one checklist row in stored order.
type checkItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

func viewTask(projectID string, task *models.Task) taskView {
	view := taskView{
		ProjectID:      projectID,
		TaskID:         task.ID,
		ShortID:        task.ShortID,
		Title:          task.Title,
		Description:    task.Description,
		State:          string(task.WorkflowState),
		Priority:       task.Priority,
		Tags:           append([]string{}, task.Tags...),
		Context:        task.Context,
		Pinned:         task.Pinned,
		Review:         task.Review,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
		StateChangedAt: task.StateChangedAt,
		CompletedAt:    task.CompletedAt,
		DueDate:        task.DueDate,
		DeferredUntil:  task.DeferredUntil,
		ResumeNote:     task.ResumeNote,
		NextCheckIn:    task.NextCheckIn,
		Checklist:      make([]checkItem, 0, len(task.Checklist)),
		Comments:       len(task.Comments),
		Attachments:    len(task.Attachments),
		Dependencies:   len(task.Dependencies),
	}
	for _, item := range task.Checklist {
		view.Checklist = append(view.Checklist, checkItem{Text: item.Text, Done: item.Done})
	}
	if task.ParentID != nil {
		view.ParentID = *task.ParentID
	}
	return view
}

// projectView is one project as a client reads it.
type projectView struct {
	ID            string   `json:"id"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Tags          []string `json:"tags"`
	Archived      bool     `json:"archived"`
	Parked        bool     `json:"parked"`
	SystemUnfiled bool     `json:"system_unfiled"`
}

func viewProject(state *config.RuntimeState, project *models.Project) projectView {
	return projectView{
		ID:            project.ID,
		Slug:          project.Slug,
		Name:          project.Name,
		Description:   project.Description,
		Tags:          append([]string{}, project.Tags...),
		Archived:      project.Archived,
		Parked:        models.ProjectTagsParked(project.Tags),
		SystemUnfiled: state.IsSystemUnfiledProject(project.ID),
	}
}

// revisions is every precondition a client may name: canonical project
// revisions by project id and raw domain artifact revisions by domain.
type revisions struct {
	Projects map[string]string `json:"projects"`
	Domains  map[string]string `json:"domains"`
}

// committed is what every successful mutation reports: the local transaction
// and the new revisions of the artifacts it touched.
type committed struct {
	TransactionID string            `json:"transaction_id"`
	Projects      map[string]string `json:"projects"`
	Domains       map[string]string `json:"domains"`
}

// domainPaths names the synced domain artifacts an expect may address.
func domainPaths(dataDir string) map[string]string {
	return map[string]string{
		"catalog":  config.WorkspaceCatalogPath(dataDir),
		"planning": config.PlanningStatePath(dataDir),
		"lenses":   config.LensDefinitionsPath(dataDir),
	}
}

// domainRevisions reads each domain artifact's raw revision; an artifact not
// yet written has none and is omitted.
func domainRevisions(dataDir string) (map[string]string, error) {
	result := make(map[string]string)
	for name, path := range domainPaths(dataDir) {
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[name] = storage.RawArtifactRevision(data)
	}
	return result, nil
}

func currentRevisions(runtime *engine.Runtime) (revisions, error) {
	snapshot, err := runtime.Snapshot()
	if err != nil {
		return revisions{}, err
	}
	projects := make(map[string]string)
	for _, project := range snapshot.Projects() {
		projects[project.ID] = snapshot.Revision(project.ID)
	}
	domains, err := domainRevisions(runtime.DataDir())
	if err != nil {
		return revisions{}, err
	}
	return revisions{Projects: projects, Domains: domains}, nil
}

// checkExpect refuses a mutation whose preconditions no longer hold, before
// anything is committed.
func checkExpect(runtime *engine.Runtime, expect *Expect) error {
	if expect == nil {
		return nil
	}
	if len(expect.Projects) > 0 {
		snapshot, err := runtime.Snapshot()
		if err != nil {
			return err
		}
		for projectID, want := range expect.Projects {
			current := snapshot.Revision(projectID)
			if current == "" {
				return newError(CodeNotFound, "expected project %s is not in the workspace", projectID).with("project_id", projectID)
			}
			if current != want {
				return newError(CodeStaleRevision, "project %s changed since it was read", projectID).
					with("project_id", projectID).with("current_revision", current)
			}
		}
	}
	if len(expect.Domains) > 0 {
		paths := domainPaths(runtime.DataDir())
		current, err := domainRevisions(runtime.DataDir())
		if err != nil {
			return err
		}
		for name, want := range expect.Domains {
			if _, known := paths[name]; !known {
				return newError(CodeInvalidRequest, "expect names unknown domain %q", name)
			}
			if current[name] != want {
				return newError(CodeStaleRevision, "domain %s changed since it was read", name).
					with("domain", name).with("current_revision", current[name])
			}
		}
	}
	return nil
}

// committedEffect reports a task commit: its transaction, the project revision
// it produced, and every domain artifact whose raw revision moved.
func committedEffect(result app.TaskResult, domainsBefore, domainsAfter map[string]string) committed {
	changed := make(map[string]string)
	for name, revision := range domainsAfter {
		if domainsBefore[name] != revision {
			changed[name] = revision
		}
	}
	return committed{
		TransactionID: result.TransactionID,
		Projects:      map[string]string{result.ProjectID: result.Revision},
		Domains:       changed,
	}
}
