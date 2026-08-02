package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
)

// V6RuntimeWorkspace is the filesystem-free runtime projection of validated
// schema-6 domains and project documents. Runtime remains an explicit adapter
// result in session 7: normal startup continues to load schema 5 until the
// guarded cutover installs this seam.
type V6RuntimeWorkspace struct {
	Runtime                *RuntimeState
	Projects               []*models.Project
	SystemUnfiledProjectID string
}

// AdaptV6ConversionPlanToRuntime adapts a complete dry-run/activation plan only
// after revalidating the exact candidate. It does not read or write the
// workspace and does not alter normal startup schema selection.
func AdaptV6ConversionPlanToRuntime(
	dataDir string,
	plan V6WorkspaceConversionPlan,
	location *time.Location,
) (*V6RuntimeWorkspace, error) {
	if err := ValidateV6WorkspaceConversionPlan(plan); err != nil {
		return nil, fmt.Errorf("adapt v6 runtime plan: %w", err)
	}
	return AdaptV6WorkspaceToRuntime(dataDir, plan.Domains.Domains, plan.Projects, location)
}

// AdaptV6WorkspaceToRuntime converts validated schema-6 persistence values to
// the current runtime structs. The conversion is deliberately explicit around
// temporary legacy names: Backlog and Doing use their compile-time schema-5
// model aliases, the protected Unfiled ID occupies RuntimeState's pre-cutover
// protected-project slot, and the independent Next list occupies the existing
// ordered planning collection without changing task lifecycle.
func AdaptV6WorkspaceToRuntime(
	dataDir string,
	domains V6DomainCandidates,
	projects []V6ProjectDocumentCandidate,
	location *time.Location,
) (*V6RuntimeWorkspace, error) {
	root, err := v6RuntimeDataDir(dataDir)
	if err != nil {
		return nil, err
	}
	if location == nil {
		location = time.Local
	}
	if err := domains.Validate(projects); err != nil {
		return nil, fmt.Errorf("adapt v6 runtime workspace: %w", err)
	}

	runtimeDomains := &Domains{
		Catalog: &WorkspaceCatalog{
			SchemaVersion:          domains.Catalog.SchemaVersion,
			WorkspaceID:            domains.Catalog.WorkspaceID,
			Projects:               make([]CatalogProject, 0, len(domains.Catalog.Projects)),
			SystemUnfiledProjectID: domains.Catalog.SystemUnfiledProjectID,
			SystemInboxProjectID:   domains.Catalog.SystemUnfiledProjectID,
		},
		Planning: &PlanningState{
			Today:     append([]ProjectTaskLocator(nil), domains.Planning.Today...),
			Next:      append([]ProjectTaskLocator(nil), domains.Planning.Next...),
			NextOrder: append([]ProjectTaskLocator(nil), domains.Planning.Next...),
		},
		Machine: &MachineState{Theme: domains.Machine.Theme},
	}
	for _, candidate := range domains.Catalog.Projects {
		runtimeDomains.Catalog.Projects = append(
			runtimeDomains.Catalog.Projects,
			v6RuntimeCatalogProject(candidate),
		)
	}

	documents := make(map[string]storage.V6ProjectDocument, len(projects))
	for _, candidate := range projects {
		documents[candidate.ProjectID] = candidate.Document
	}
	runtimeProjects := make([]*models.Project, 0, len(domains.Catalog.Projects))
	for _, catalogProject := range domains.Catalog.Projects {
		project, adaptErr := adaptV6RuntimeProject(
			root,
			catalogProject,
			documents[catalogProject.ID],
			location,
		)
		if adaptErr != nil {
			return nil, adaptErr
		}
		runtimeProjects = append(runtimeProjects, project)
	}

	return &V6RuntimeWorkspace{
		Runtime:                NewRuntimeState(root, runtimeDomains),
		Projects:               runtimeProjects,
		SystemUnfiledProjectID: domains.Catalog.SystemUnfiledProjectID,
	}, nil
}

func v6RuntimeDataDir(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("adapt v6 runtime workspace: data directory is required")
	}
	root, err := filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return "", fmt.Errorf("adapt v6 runtime workspace data directory: %w", err)
	}
	return root, nil
}

func v6RuntimeCatalogProject(candidate V6CatalogProjectCandidate) CatalogProject {
	return CatalogProject{
		ID:          candidate.ID,
		Slug:        candidate.Slug,
		Name:        candidate.Name,
		Description: candidate.Description,
		Context:     candidate.Context,
		CreatedAt:   candidate.CreatedAt,
		UpdatedAt:   candidate.UpdatedAt,
		Archived:    candidate.Archived,
		ArchivedAt:  cloneV6RuntimeTime(candidate.ArchivedAt),
		Pinned:      candidate.Pinned,
		PinnedAt:    cloneV6RuntimeTime(candidate.PinnedAt),
		ColorSlot:   candidate.ColorSlot,
	}
}

func adaptV6RuntimeProject(
	dataDir string,
	candidate V6CatalogProjectCandidate,
	document storage.V6ProjectDocument,
	location *time.Location,
) (*models.Project, error) {
	project := &models.Project{
		ID:          candidate.ID,
		Slug:        candidate.Slug,
		Name:        candidate.Name,
		Description: candidate.Description,
		Context:     candidate.Context,
		Path:        filepath.Join(dataDir, "projects", candidate.Slug),
		CreatedAt:   candidate.CreatedAt,
		UpdatedAt:   candidate.UpdatedAt,
		Archived:    candidate.Archived,
		ArchivedAt:  cloneV6RuntimeTime(candidate.ArchivedAt),
		Pinned:      candidate.Pinned,
		PinnedAt:    cloneV6RuntimeTime(candidate.PinnedAt),
		ColorSlot:   candidate.ColorSlot,
		Tasks:       make([]*models.Task, 0, len(document.Tasks)),
		NextShortID: 1,
	}
	for _, record := range document.Tasks {
		task, err := adaptV6RuntimeTask(record, location)
		if err != nil {
			return nil, fmt.Errorf(
				"adapt v6 runtime project %q task %q: %w",
				candidate.ID,
				record.ID,
				err,
			)
		}
		task.Project = project
		project.Tasks = append(project.Tasks, task)
		if task.ShortID >= project.NextShortID {
			project.NextShortID = task.ShortID + 1
		}
	}
	project.BuildTaskTree()
	return project, nil
}

func adaptV6RuntimeTask(record format.V6TaskRecord, location *time.Location) (*models.Task, error) {
	state, err := v6RuntimeWorkflowState(record.State)
	if err != nil {
		return nil, err
	}
	task := &models.Task{
		ID:            record.ID,
		ShortID:       record.ShortID,
		Title:         record.Title,
		Description:   record.Description,
		WorkflowState: state,
		Priority:      record.Priority,
		Tags:          append([]string(nil), record.Tags...),
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
		CompletedAt:   cloneV6RuntimeTime(record.CompletedAt),
		Pinned:        record.Pinned,
		PinnedAt:      cloneV6RuntimeTime(record.PinnedAt),
		Comments:      make([]*models.Comment, 0, len(record.Comments)),
		Decisions:     []*models.Decision{},
		Attachments:   make([]models.Attachment, 0, len(record.Attachments)),
		Checklist:     make([]models.ChecklistItem, 0, len(record.Checklist)),
		Children:      []*models.Task{},
	}
	if record.ParentID != "" {
		parentID := record.ParentID
		task.ParentID = &parentID
	}
	if record.DueDate != "" {
		task.DueDate, err = parseV6RuntimeDate(record.DueDate, location)
		if err != nil {
			return nil, fmt.Errorf("due date: %w", err)
		}
	}
	if record.SnoozedUntil != "" {
		task.SnoozedUntil, err = parseV6RuntimeDate(record.SnoozedUntil, location)
		if err != nil {
			return nil, fmt.Errorf("snooze date: %w", err)
		}
	}
	for _, attachment := range record.Attachments {
		modelAttachment := models.Attachment{
			Filename:     attachment.Filename,
			IsURL:        attachment.Kind == "url",
			IsCodeRef:    attachment.Kind == "code",
			LineNumber:   attachment.LineNumber,
			OriginalPath: attachment.OriginalPath,
		}
		if attachment.AddedAt != nil {
			modelAttachment.AddedAt = *attachment.AddedAt
		}
		task.Attachments = append(task.Attachments, modelAttachment)
	}
	for _, item := range record.Checklist {
		task.Checklist = append(task.Checklist, models.ChecklistItem{Text: item.Text, Done: item.Done})
	}
	for _, comment := range record.Comments {
		task.Comments = append(task.Comments, &models.Comment{Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	return task, nil
}

func v6RuntimeWorkflowState(state string) (models.WorkflowState, error) {
	switch state {
	case "inbox":
		return models.StateInbox, nil
	case "backlog":
		return models.StateBacklog, nil
	case "doing":
		return models.StateDoing, nil
	case "waiting":
		return models.StateWaiting, nil
	case "paused":
		return models.StatePaused, nil
	case "someday":
		return models.StateSomeday, nil
	case "done":
		return models.StateDone, nil
	case "canceled":
		return models.StateCanceled, nil
	default:
		return "", fmt.Errorf("invalid schema-6 workflow state %q", state)
	}
}

func parseV6RuntimeDate(value string, location *time.Location) (*time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func cloneV6RuntimeTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
