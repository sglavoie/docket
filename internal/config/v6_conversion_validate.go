package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
)

// ValidateV6WorkspaceConversionPlan verifies the complete in-memory candidate
// graph without consulting live schema-5 state or writing target paths.
func ValidateV6WorkspaceConversionPlan(plan V6WorkspaceConversionPlan) error {
	if err := plan.Domains.Domains.Validate(plan.Projects); err != nil {
		return err
	}
	if err := validateV6WorkspaceArtifacts(plan); err != nil {
		return err
	}
	if err := validateV6AttachmentCandidates(plan); err != nil {
		return err
	}
	return nil
}

// Validate verifies all schema-6 domain payloads against every project
// document candidate.
func (domains V6DomainCandidates) Validate(projects []V6ProjectDocumentCandidate) error {
	if err := domains.Catalog.Validate(); err != nil {
		return err
	}
	if err := validateV6ProjectCandidates(domains.Catalog, projects); err != nil {
		return err
	}
	if err := domains.Planning.Validate(domains.Catalog, projects); err != nil {
		return err
	}
	if err := domains.Machine.Validate(); err != nil {
		return err
	}
	return nil
}

func (catalog V6WorkspaceCatalogCandidate) Validate() error {
	if catalog.SchemaVersion != V6ConfigSchemaVersion {
		return fmt.Errorf("v6 catalog schema is %d, want %d", catalog.SchemaVersion, V6ConfigSchemaVersion)
	}
	if err := validateWorkspaceID(catalog.WorkspaceID); err != nil {
		return fmt.Errorf("v6 catalog: %w", err)
	}
	ids := make(map[string]bool, len(catalog.Projects))
	slugs := make(map[string]bool, len(catalog.Projects))
	for index, project := range catalog.Projects {
		if err := validateV6CatalogProject(project); err != nil {
			return fmt.Errorf("v6 catalog project %d: %w", index, err)
		}
		if ids[project.ID] {
			return fmt.Errorf("v6 catalog has duplicate project ID %q", project.ID)
		}
		if slugs[project.Slug] {
			return fmt.Errorf("v6 catalog has duplicate project slug %q", project.Slug)
		}
		ids[project.ID] = true
		slugs[project.Slug] = true
	}
	if catalog.SystemUnfiledProjectID == "" || !ids[catalog.SystemUnfiledProjectID] {
		return fmt.Errorf("v6 catalog protected Unfiled project is missing or unknown")
	}
	for _, project := range catalog.Projects {
		if project.ID != catalog.SystemUnfiledProjectID {
			continue
		}
		if project.Name != V6UnfiledProjectName || project.Slug != V6UnfiledProjectSlug {
			return fmt.Errorf(
				"v6 protected project %q must be %s/%s",
				project.ID, V6UnfiledProjectName, V6UnfiledProjectSlug,
			)
		}
		if project.Archived || project.ArchivedAt != nil {
			return fmt.Errorf("v6 protected Unfiled project %q cannot be archived", project.ID)
		}
	}
	return nil
}

func validateV6CatalogProject(project V6CatalogProjectCandidate) error {
	if strings.TrimSpace(project.ID) == "" || project.ID != strings.TrimSpace(project.ID) {
		return fmt.Errorf("project has an empty or untrimmed ID")
	}
	if err := models.ValidateProjectSlug(project.Slug); err != nil {
		return fmt.Errorf("project %q: %w", project.ID, err)
	}
	if strings.TrimSpace(project.Name) == "" || project.Name != strings.TrimSpace(project.Name) {
		return fmt.Errorf("project %q requires a trimmed name", project.ID)
	}
	if len(project.Name) > models.MaxProjectNameLength || strings.ContainsAny(project.Name, "\n\r") {
		return fmt.Errorf("project %q has an invalid name", project.ID)
	}
	if strings.TrimSpace(project.Context) == "" || project.Context != strings.TrimSpace(project.Context) {
		return fmt.Errorf("project %q requires a regular context", project.ID)
	}
	if project.CreatedAt.IsZero() || project.UpdatedAt.IsZero() {
		return fmt.Errorf("project %q requires created and updated timestamps", project.ID)
	}
	if project.Archived != (project.ArchivedAt != nil) {
		return fmt.Errorf("project %q has inconsistent archive state", project.ID)
	}
	if project.Pinned != (project.PinnedAt != nil) {
		return fmt.Errorf("project %q has inconsistent pin state", project.ID)
	}
	if !validProjectColorSlot(project.ColorSlot) {
		return fmt.Errorf("project %q has invalid color slot %d", project.ID, project.ColorSlot)
	}
	return nil
}

func validateV6ProjectCandidates(
	catalog V6WorkspaceCatalogCandidate,
	projects []V6ProjectDocumentCandidate,
) error {
	byID := make(map[string]V6CatalogProjectCandidate, len(catalog.Projects))
	for _, project := range catalog.Projects {
		byID[project.ID] = project
	}
	seen := make(map[string]bool, len(projects))
	for _, candidate := range projects {
		project, exists := byID[candidate.ProjectID]
		if !exists {
			return fmt.Errorf("v6 document candidate references unknown project %q", candidate.ProjectID)
		}
		if seen[candidate.ProjectID] {
			return fmt.Errorf("v6 conversion repeats document candidate for project %q", candidate.ProjectID)
		}
		seen[candidate.ProjectID] = true
		if candidate.Document.ProjectName != project.Name {
			return fmt.Errorf(
				"v6 project %q document name %q does not match catalog name %q",
				candidate.ProjectID, candidate.Document.ProjectName, project.Name,
			)
		}
		if err := storage.ValidateV6ProjectDocument(candidate.Document); err != nil {
			return fmt.Errorf("validate v6 project %q document: %w", candidate.ProjectID, err)
		}
		if strings.TrimSpace(candidate.SourcePath) == "" || strings.TrimSpace(candidate.Path) == "" {
			return fmt.Errorf("v6 project %q has incomplete artifact paths", candidate.ProjectID)
		}
		if !candidate.SourceExists && !project.Archived {
			return fmt.Errorf("active v6 project %q has no source document", candidate.ProjectID)
		}
	}
	for _, project := range catalog.Projects {
		if !seen[project.ID] {
			return fmt.Errorf("v6 conversion produced no document candidate for project %q", project.ID)
		}
	}
	return nil
}

func (planning V6PlanningCandidate) Validate(
	catalog V6WorkspaceCatalogCandidate,
	projects []V6ProjectDocumentCandidate,
) error {
	projectIDs := make(map[string]bool, len(catalog.Projects))
	for _, project := range catalog.Projects {
		projectIDs[project.ID] = true
	}
	tasks := make(map[ProjectTaskLocator]format.V6TaskRecord)
	for _, project := range projects {
		for _, task := range project.Document.Tasks {
			tasks[ProjectTaskLocator{ProjectID: project.ProjectID, TaskID: task.ID}] = task
		}
	}
	seenToday := make(map[ProjectTaskLocator]bool, len(planning.Today))
	for index, ref := range planning.Today {
		if !ref.Valid() || !projectIDs[ref.ProjectID] {
			return fmt.Errorf("v6 planning Today item %d has invalid identity", index)
		}
		task, exists := tasks[ref]
		if !exists {
			return fmt.Errorf(
				"v6 planning Today item %d references unknown task %q in project %q",
				index, ref.TaskID, ref.ProjectID,
			)
		}
		if v6TerminalState(task.State) {
			return fmt.Errorf("v6 planning Today contains terminal task %q", ref.TaskID)
		}
		if seenToday[ref] {
			return fmt.Errorf("v6 planning Today repeats task %q in project %q", ref.TaskID, ref.ProjectID)
		}
		seenToday[ref] = true
	}
	seenNext := make(map[ProjectTaskLocator]bool, len(planning.Next))
	for index, ref := range planning.Next {
		if !ref.Valid() || !projectIDs[ref.ProjectID] {
			return fmt.Errorf("v6 planning Next item %d has invalid identity", index)
		}
		task, exists := tasks[ref]
		if !exists {
			return fmt.Errorf(
				"v6 planning Next item %d references unknown task %q in project %q",
				index, ref.TaskID, ref.ProjectID,
			)
		}
		if v6TerminalState(task.State) {
			return fmt.Errorf("v6 planning Next contains terminal task %q", ref.TaskID)
		}
		if seenNext[ref] {
			return fmt.Errorf("v6 planning Next repeats task %q in project %q", ref.TaskID, ref.ProjectID)
		}
		if seenToday[ref] {
			return fmt.Errorf("v6 planning task %q in project %q belongs to Today and Next", ref.TaskID, ref.ProjectID)
		}
		seenNext[ref] = true
	}
	return nil
}

func (machine V6MachineCandidate) Validate() error {
	if machine.Theme != "dark" && machine.Theme != "light" {
		return fmt.Errorf("v6 machine theme %q is invalid", machine.Theme)
	}
	return nil
}

func validateV6WorkspaceArtifacts(plan V6WorkspaceConversionPlan) error {
	targetPaths := make(map[string]bool, len(plan.Artifacts))
	domainKinds := make(map[string]bool, 3)
	projectArtifacts := make(map[string]V6WorkspaceArtifactCandidate, len(plan.Projects))
	projectCandidates := make(map[string]V6ProjectDocumentCandidate, len(plan.Projects))
	for _, project := range plan.Projects {
		projectCandidates[project.ProjectID] = project
	}
	for _, artifact := range plan.Artifacts {
		path := filepath.Clean(artifact.Path)
		if strings.TrimSpace(artifact.Path) == "" || strings.TrimSpace(artifact.SourcePath) == "" {
			return fmt.Errorf("v6 %s artifact has an incomplete path", artifact.Kind)
		}
		if targetPaths[path] {
			return fmt.Errorf("v6 conversion produced duplicate target path %s", path)
		}
		targetPaths[path] = true
		switch artifact.Kind {
		case "workspace", "planning", "machine":
			if domainKinds[artifact.Kind] {
				return fmt.Errorf("v6 conversion produced duplicate %s domain", artifact.Kind)
			}
			domainKinds[artifact.Kind] = true
			if err := validateV6DomainCandidateBytes(
				artifact.Kind, plan.Domains.Domains.Catalog.WorkspaceID, artifact.Data, plan.Domains.Domains,
			); err != nil {
				return err
			}
		case "project":
			candidate, exists := projectCandidates[artifact.ProjectID]
			if !exists {
				return fmt.Errorf("v6 project artifact references unknown project %q", artifact.ProjectID)
			}
			if _, duplicate := projectArtifacts[artifact.ProjectID]; duplicate {
				return fmt.Errorf("v6 conversion repeats project artifact %q", artifact.ProjectID)
			}
			if candidate.Path != artifact.Path || candidate.SourcePath != artifact.SourcePath ||
				candidate.SourceExists != artifact.SourceExists {
				return fmt.Errorf("v6 project %q candidate paths disagree with its artifact", artifact.ProjectID)
			}
			_, err := storage.ParseV6ProjectDocument(string(artifact.Data))
			if err != nil {
				return fmt.Errorf("validate v6 project %q artifact: %w", artifact.ProjectID, err)
			}
			candidateData, err := storage.WriteV6ProjectDocument(candidate.Document)
			if err != nil || candidateData != string(artifact.Data) {
				return fmt.Errorf("v6 project %q artifact differs from its document candidate", artifact.ProjectID)
			}
			projectArtifacts[artifact.ProjectID] = artifact
		case "global-note", "project-note":
			if !artifact.SourceExists {
				return fmt.Errorf("retained v6 note artifact %s has no source", artifact.SourcePath)
			}
			if artifact.Kind == "project-note" {
				if _, exists := projectCandidates[artifact.ProjectID]; !exists {
					return fmt.Errorf("v6 note artifact references unknown project %q", artifact.ProjectID)
				}
			} else if artifact.ProjectID != "" {
				return fmt.Errorf("global v6 note unexpectedly references project %q", artifact.ProjectID)
			}
			if err := storage.ValidateV5NoteContent(filepath.Base(path), artifact.Data); err != nil {
				return err
			}
		default:
			return fmt.Errorf("v6 conversion produced unknown artifact kind %q", artifact.Kind)
		}
	}
	for _, kind := range []string{"workspace", "planning", "machine"} {
		if !domainKinds[kind] {
			return fmt.Errorf("v6 conversion did not produce a %s domain", kind)
		}
	}
	for _, project := range plan.Projects {
		if _, exists := projectArtifacts[project.ProjectID]; !exists {
			return fmt.Errorf("v6 conversion produced no artifact for project %q", project.ProjectID)
		}
	}
	return nil
}

func validateV6DomainCandidateBytes(
	kind string,
	workspaceID string,
	data []byte,
	expected V6DomainCandidates,
) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope domainEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("validate v6 %s envelope: %w", kind, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return fmt.Errorf("validate v6 %s envelope: %w", kind, err)
	}
	if envelope.SchemaVersion != CurrentDomainSchemaVersion ||
		envelope.WorkspaceID != workspaceID || envelope.Kind != kind {
		return fmt.Errorf("validate v6 %s envelope: identity mismatch", kind)
	}
	var got any
	var want any
	switch kind {
	case "workspace":
		got = &V6WorkspaceCatalogCandidate{}
		want = &expected.Catalog
	case "planning":
		got = &V6PlanningCandidate{}
		want = &expected.Planning
	case "machine":
		got = &V6MachineCandidate{}
		want = &expected.Machine
	default:
		return fmt.Errorf("validate v6 unknown domain kind %q", kind)
	}
	payload := json.NewDecoder(bytes.NewReader(envelope.Data))
	payload.DisallowUnknownFields()
	if err := payload.Decode(got); err != nil {
		return fmt.Errorf("validate v6 %s payload: %w", kind, err)
	}
	if err := requireJSONEOF(payload); err != nil {
		return fmt.Errorf("validate v6 %s payload: %w", kind, err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		return fmt.Errorf("validate v6 %s decoded payload: %w", kind, err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		return fmt.Errorf("validate v6 %s expected payload: %w", kind, err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		return fmt.Errorf("validate v6 %s payload: candidate mismatch", kind)
	}
	return nil
}

func validateV6AttachmentCandidates(plan V6WorkspaceConversionPlan) error {
	tasks := make(map[ProjectTaskLocator]format.V6TaskRecord)
	projectAttachmentRoots := make(map[string]string, len(plan.Projects))
	expected := make(map[string]bool)
	for _, project := range plan.Projects {
		projectAttachmentRoots[project.ProjectID] = filepath.Join(filepath.Dir(project.SourcePath), ".attachments")
		for _, task := range project.Document.Tasks {
			locator := ProjectTaskLocator{ProjectID: project.ProjectID, TaskID: task.ID}
			tasks[locator] = task
			for _, attachment := range task.Attachments {
				if attachment.Kind != "file" {
					continue
				}
				key := v6AttachmentKey(project.ProjectID, task.ID, attachment.Filename)
				if expected[key] {
					return fmt.Errorf(
						"v6 task %q in project %q repeats local attachment %q",
						task.ID, project.ProjectID, attachment.Filename,
					)
				}
				expected[key] = true
			}
		}
	}
	seen := make(map[string]bool, len(plan.Attachments))
	sourcePaths := make(map[string]bool, len(plan.Attachments))
	targetPaths := make(map[string]bool, len(plan.Attachments))
	for _, candidate := range plan.Attachments {
		locator := ProjectTaskLocator{ProjectID: candidate.ProjectID, TaskID: candidate.TaskID}
		if _, exists := tasks[locator]; !exists {
			return fmt.Errorf(
				"v6 attachment %q references unknown task %q in project %q",
				candidate.Filename, candidate.TaskID, candidate.ProjectID,
			)
		}
		key := v6AttachmentKey(candidate.ProjectID, candidate.TaskID, candidate.Filename)
		if !expected[key] {
			return fmt.Errorf("v6 attachment candidate %q has no local attachment metadata", candidate.SourcePath)
		}
		if seen[key] {
			return fmt.Errorf("v6 conversion repeats attachment candidate for %s", key)
		}
		seen[key] = true
		if filepath.Base(candidate.SourcePath) != candidate.Filename || filepath.Base(candidate.Path) != candidate.Filename {
			return fmt.Errorf("v6 attachment candidate %s has inconsistent filenames", key)
		}
		if filepath.Base(filepath.Dir(candidate.Path)) != v6AttachmentTaskDirName(candidate.TaskID) {
			return fmt.Errorf("v6 attachment candidate %s has a non-canonical target directory", key)
		}
		sourcePath, targetPath := filepath.Clean(candidate.SourcePath), filepath.Clean(candidate.Path)
		if sourcePaths[sourcePath] || targetPaths[targetPath] {
			return fmt.Errorf("v6 conversion repeats attachment source or target path for %s", key)
		}
		sourcePaths[sourcePath] = true
		targetPaths[targetPath] = true
		if candidate.Size < 0 || len(candidate.SHA256) != sha256.Size*2 {
			return fmt.Errorf("v6 attachment candidate %s has invalid size or digest", key)
		}
		if _, err := hex.DecodeString(candidate.SHA256); err != nil {
			return fmt.Errorf("v6 attachment candidate %s has invalid digest: %w", key, err)
		}
	}
	for key := range expected {
		if !seen[key] {
			return fmt.Errorf("v6 conversion produced no retained attachment candidate for %s", key)
		}
	}
	obsoletePaths := make(map[string]bool, len(plan.ObsoleteAttachments))
	for _, candidate := range plan.ObsoleteAttachments {
		path := filepath.Clean(candidate.Path)
		if strings.TrimSpace(candidate.ProjectID) == "" || strings.TrimSpace(candidate.Path) == "" {
			return fmt.Errorf("v6 obsolete attachment candidate has incomplete identity")
		}
		root, exists := projectAttachmentRoots[candidate.ProjectID]
		if !exists || filepath.Dir(path) != root {
			return fmt.Errorf(
				"v6 obsolete attachment candidate is outside project %q attachment root: %s",
				candidate.ProjectID,
				path,
			)
		}
		if obsoletePaths[path] {
			return fmt.Errorf("v6 conversion repeats obsolete attachment directory %s", path)
		}
		for sourcePath := range sourcePaths {
			if filepath.Dir(sourcePath) == path {
				return fmt.Errorf("v6 attachment directory is both retained and obsolete: %s", path)
			}
		}
		obsoletePaths[path] = true
	}
	return nil
}

func v6AttachmentKey(projectID, taskID, filename string) string {
	return projectID + "\x00" + taskID + "\x00" + filename
}
