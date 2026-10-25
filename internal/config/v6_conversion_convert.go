package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/storage/format"
)

func convertV6CatalogCandidate(
	root string,
	source V5WorkspaceCatalogCandidate,
) (V6WorkspaceCatalogCandidate, []V6DomainMigrationAuditEntry, error) {
	protectedIndex := -1
	for index, project := range source.Projects {
		if project.ID == source.SystemInboxProjectID {
			protectedIndex = index
			continue
		}
		if project.Slug == V6UnfiledProjectSlug {
			return V6WorkspaceCatalogCandidate{}, nil, fmt.Errorf(
				"schema-6 Unfiled slug %q is already owned by project %q",
				V6UnfiledProjectSlug, project.ID,
			)
		}
	}
	if protectedIndex < 0 {
		return V6WorkspaceCatalogCandidate{}, nil, fmt.Errorf(
			"v5 protected system Inbox %q is missing from the catalog", source.SystemInboxProjectID,
		)
	}
	protected := source.Projects[protectedIndex]
	if protected.Slug != V6UnfiledProjectSlug {
		collisionPath := filepath.Join(root, "projects", V6UnfiledProjectSlug)
		if _, err := os.Lstat(collisionPath); err == nil {
			return V6WorkspaceCatalogCandidate{}, nil, fmt.Errorf(
				"schema-6 Unfiled filesystem target already exists: %s", collisionPath,
			)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return V6WorkspaceCatalogCandidate{}, nil, fmt.Errorf(
				"inspect schema-6 Unfiled filesystem target %s: %w", collisionPath, err,
			)
		}
	}

	target := V6WorkspaceCatalogCandidate{
		SchemaVersion:          V6ConfigSchemaVersion,
		WorkspaceID:            source.WorkspaceID,
		SystemUnfiledProjectID: source.SystemInboxProjectID,
		Projects:               make([]V6CatalogProjectCandidate, 0, len(source.Projects)),
	}
	for _, project := range source.Projects {
		candidate := V6CatalogProjectCandidate{
			ID:          project.ID,
			Slug:        project.Slug,
			Name:        project.Name,
			Description: project.Description,
			Context:     project.Context,
			CreatedAt:   project.CreatedAt,
			UpdatedAt:   project.UpdatedAt,
			Archived:    project.Archived,
			ArchivedAt:  cloneTimePointer(project.ArchivedAt),
			Pinned:      project.Pinned,
			PinnedAt:    cloneTimePointer(project.PinnedAt),
			ColorSlot:   project.ColorSlot,
		}
		if project.ID == source.SystemInboxProjectID {
			candidate.Name = V6UnfiledProjectName
			candidate.Slug = V6UnfiledProjectSlug
		}
		target.Projects = append(target.Projects, candidate)
	}

	var audit []V6DomainMigrationAuditEntry
	appendChange := func(field, from, to, reason string) {
		if from == to {
			return
		}
		audit = append(audit, V6DomainMigrationAuditEntry{
			Domain: "workspace", ProjectID: protected.ID, EntityID: protected.ID,
			Action: "converted", Field: field, From: from, To: to, Reason: reason,
		})
	}
	appendChange(
		"project_name", protected.Name, V6UnfiledProjectName,
		"the protected capture project is named Unfiled in schema 6",
	)
	appendChange(
		"project_slug", protected.Slug, V6UnfiledProjectSlug,
		"the protected capture project uses the unfiled filesystem slug in schema 6",
	)
	audit = append(audit, V6DomainMigrationAuditEntry{
		Domain: "workspace", ProjectID: protected.ID, EntityID: protected.ID,
		Action: "renamed", Field: "protected_project_reference",
		From: "system_inbox_project_id", To: "system_unfiled_project_id",
		Reason: "schema 6 renames the protected catalog reference while preserving its project ID",
	})
	if err := target.Validate(); err != nil {
		return V6WorkspaceCatalogCandidate{}, nil, err
	}
	return target, audit, nil
}

//nolint:govet // Per-task conversion errors are intentionally scoped inside the conversion loop.
func convertV6ProjectCandidate(
	root string,
	sourceProject V5ProjectCandidate,
	targetProject V6CatalogProjectCandidate,
) (
	V6ProjectDocumentCandidate,
	V6WorkspaceArtifactCandidate,
	[]v6CandidateTask,
	[]format.V6TaskMigrationAuditEntry,
	error,
) {
	sourceProjectDir := filepath.Join(root, "projects", sourceProject.Slug)
	if err := validateV6SourceProjectDir(sourceProjectDir, sourceProject.Archived); err != nil {
		return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, err
	}
	sourcePath := storage.TasksFilePath(sourceProjectDir)
	targetPath := storage.TasksFilePath(filepath.Join(root, "projects", targetProject.Slug))
	data, found, err := readOptionalV5DryRunSource(sourcePath)
	if err != nil {
		return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, err
	}
	var sourceDocument storage.V5ProjectDocument
	if found {
		sourceDocument, err = storage.ParseV5ProjectDocument(string(data))
		if err != nil {
			return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, fmt.Errorf(
				"parse v5 project %q task document: %w", sourceProject.ID, err,
			)
		}
		if sourceDocument.ProjectName != sourceProject.Name {
			return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, fmt.Errorf(
				"project %q task document name %q does not match catalog name %q",
				sourceProject.ID, sourceDocument.ProjectName, sourceProject.Name,
			)
		}
	} else {
		if !sourceProject.Archived {
			return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, fmt.Errorf(
				"active project %q has no v5 task document at %s", sourceProject.ID, sourcePath,
			)
		}
		sourceDocument = storage.V5ProjectDocument{ProjectName: sourceProject.Name}
	}

	targetDocument := storage.V6ProjectDocument{
		ProjectName: targetProject.Name,
	}
	if len(sourceDocument.Tasks) > 0 {
		targetDocument.Tasks = make([]format.V6TaskRecord, 0, len(sourceDocument.Tasks))
	}
	convertedTasks := make([]v6CandidateTask, 0, len(sourceDocument.Tasks))
	var taskAudit []format.V6TaskMigrationAuditEntry
	for _, sourceTask := range sourceDocument.Tasks {
		converted, err := format.ConvertV5Task(sourceTask)
		if err != nil {
			return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, err
		}
		targetDocument.Tasks = append(targetDocument.Tasks, converted.Task)
		convertedTasks = append(convertedTasks, v6CandidateTask{
			ProjectID: targetProject.ID, Task: converted.Task, Next: converted.NextMember,
		})
		for index := range converted.Audit {
			converted.Audit[index].ProjectID = targetProject.ID
		}
		taskAudit = append(taskAudit, converted.Audit...)
	}
	content, err := storage.WriteV6ProjectDocument(targetDocument)
	if err != nil {
		return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, err
	}
	parsed, err := storage.ParseV6ProjectDocument(content)
	if err != nil {
		return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, fmt.Errorf(
			"validate v6 project %q target: %w", targetProject.ID, err,
		)
	}
	rendered, err := storage.WriteV6ProjectDocument(parsed)
	if err != nil || rendered != content {
		return V6ProjectDocumentCandidate{}, V6WorkspaceArtifactCandidate{}, nil, nil, fmt.Errorf(
			"v6 project %q target did not round trip", targetProject.ID,
		)
	}
	targetDocument = parsed
	documentCandidate := V6ProjectDocumentCandidate{
		ProjectID: targetProject.ID, SourcePath: sourcePath, Path: targetPath,
		SourceExists: found, Document: targetDocument,
	}
	artifactCandidate := V6WorkspaceArtifactCandidate{
		SourcePath: sourcePath, Path: targetPath, Kind: "project",
		ProjectID: targetProject.ID, SourceExists: found, Data: []byte(content),
	}
	return documentCandidate, artifactCandidate, convertedTasks, taskAudit, nil
}

func validateV6SourceProjectDir(path string, archived bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) && archived {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect v5 project source %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("v5 project source is not a regular directory: %s", path)
	}
	return nil
}

func convertV6PlanningCandidate(
	source V5PlanningCandidate,
	tasks map[ProjectTaskLocator]v6CandidateTask,
	orderedTasks []v6CandidateTask,
) (V6PlanningCandidate, []V6DomainMigrationAuditEntry) {
	var target V6PlanningCandidate
	var audit []V6DomainMigrationAuditEntry
	appendAudit := func(ref ProjectTaskLocator, action, field, reason string) {
		audit = append(audit, V6DomainMigrationAuditEntry{
			Domain: "planning", ProjectID: ref.ProjectID, EntityID: ref.TaskID,
			Action: action, Field: field,
			From: ref.ProjectID + "/" + ref.TaskID, Reason: reason,
		})
	}

	seenNext := make(map[ProjectTaskLocator]bool, len(source.NextOrder))
	for _, ref := range source.NextOrder {
		task, exists := tasks[ref]
		switch {
		case !exists:
			appendAudit(ref, "removed", "next", "dangling v5 NextOrder reference has no retained task")
		case !task.Next:
			appendAudit(ref, "removed", "next", "v5 NextOrder did not identify a StateNext task")
		case seenNext[ref]:
			appendAudit(ref, "removed", "next", "duplicate v5 NextOrder membership was removed")
		default:
			seenNext[ref] = true
			target.Next = append(target.Next, ref)
		}
	}
	for _, task := range orderedTasks {
		if !task.Next {
			continue
		}
		ref := ProjectTaskLocator{ProjectID: task.ProjectID, TaskID: task.Task.ID}
		if seenNext[ref] {
			continue
		}
		seenNext[ref] = true
		target.Next = append(target.Next, ref)
		appendAudit(ref, "appended", "next", "StateNext membership missing from v5 NextOrder was appended in catalog and document order")
	}

	seenToday := make(map[ProjectTaskLocator]bool, len(source.Today))
	for _, ref := range source.Today {
		task, exists := tasks[ref]
		switch {
		case !exists:
			appendAudit(ref, "removed", "today", "dangling v5 Today reference has no retained task")
		case task.Next:
			appendAudit(ref, "removed", "today", "schema-6 Next membership wins a Today and Next conflict")
		case v6TerminalState(task.Task.State):
			appendAudit(ref, "removed", "today", "terminal tasks cannot belong to schema-6 Today")
		case seenToday[ref]:
			appendAudit(ref, "removed", "today", "duplicate v5 Today membership was removed")
		default:
			seenToday[ref] = true
			target.Today = append(target.Today, ref)
		}
	}
	return target, audit
}

func convertV6MachineCandidate(source V5MachineCandidate) (V6MachineCandidate, []V6DomainMigrationAuditEntry) {
	target := V6MachineCandidate{Theme: source.Theme}
	names := make([]string, 0, len(source.CustomContexts))
	for name := range source.CustomContexts {
		names = append(names, name)
	}
	sort.Slice(names, func(left, right int) bool {
		leftFolded, rightFolded := strings.ToLower(names[left]), strings.ToLower(names[right])
		if leftFolded != rightFolded {
			return leftFolded < rightFolded
		}
		return names[left] < names[right]
	})
	audit := make([]V6DomainMigrationAuditEntry, 0, len(names))
	for _, name := range names {
		audit = append(audit, V6DomainMigrationAuditEntry{
			Domain: "machine", EntityID: name, Action: "discarded", Field: "custom_context",
			From:   fmt.Sprintf("%d members", len(source.CustomContexts[name])),
			Reason: "schema 6 removes machine-local custom contexts",
		})
	}
	return target, audit
}

func v6TerminalState(state string) bool {
	return state == "done" || state == "canceled"
}
