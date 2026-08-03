package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
)

const (
	// V6ConfigSchemaVersion is the independently validated configuration
	// payload version produced by this dry run. Normal startup remains on v5
	// until the explicit live cutover.
	V6ConfigSchemaVersion = 6

	V6UnfiledProjectName = "Unfiled"
	V6UnfiledProjectSlug = "unfiled"
)

// V6WorkspaceDryRunOptions identifies the schema-5 workspace and optionally
// supplies an isolated machine-state path for migration rehearsals.
type V6WorkspaceDryRunOptions struct {
	DataDir          string
	MachineStatePath string
}

// V6CatalogProjectCandidate is the retained project shape in schema 6.
type V6CatalogProjectCandidate struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Context     string     `json:"context"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	ColorSlot   int        `json:"color_slot"`
}

// V6WorkspaceCatalogCandidate renames the protected reference while retaining
// the stable project and workspace identities.
type V6WorkspaceCatalogCandidate struct {
	SchemaVersion          int                         `json:"schema_version"`
	WorkspaceID            string                      `json:"workspace_id"`
	Projects               []V6CatalogProjectCandidate `json:"projects"`
	SystemUnfiledProjectID string                      `json:"system_unfiled_project_id"`
}

type V6PlanningTaskRef = ProjectTaskLocator

// V6PlanningCandidate owns two explicit, ordered membership lists. Lifecycle
// state no longer carries Next membership.
type V6PlanningCandidate struct {
	Today []V6PlanningTaskRef `json:"today,omitempty"`
	Next  []V6PlanningTaskRef `json:"next,omitempty"`
}

// V6MachineCandidate retains only the selected built-in theme.
type V6MachineCandidate struct {
	Theme string `json:"theme"`
}

type V6DomainCandidates struct {
	Catalog  V6WorkspaceCatalogCandidate
	Planning V6PlanningCandidate
	Machine  V6MachineCandidate
}

// V6DomainMigrationAuditEntry records deterministic catalog, planning, and
// machine repairs made while assembling the schema-6 candidate.
type V6DomainMigrationAuditEntry struct {
	Domain    string `json:"domain"`
	ProjectID string `json:"project_id,omitempty"`
	EntityID  string `json:"entity_id,omitempty"`
	Action    string `json:"action"`
	Field     string `json:"field"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Reason    string `json:"reason"`
}

type V6DomainConversionResult struct {
	Domains V6DomainCandidates
	Audit   []V6DomainMigrationAuditEntry
}

// V6ProjectDocumentCandidate pairs one validated schema-6 document with its
// source and eventual target paths. Archived projects with no source task file
// receive an explicit empty candidate.
type V6ProjectDocumentCandidate struct {
	ProjectID    string
	SourcePath   string
	Path         string
	SourceExists bool
	Document     storage.V6ProjectDocument
}

// V6WorkspaceArtifactCandidate contains exact target bytes for domains,
// project documents, and retained notes. SourcePath may differ from Path when
// the protected Inbox directory becomes unfiled.
type V6WorkspaceArtifactCandidate struct {
	SourcePath   string
	Path         string
	Kind         string
	ProjectID    string
	SourceExists bool
	Data         []byte
}

// V6RetainedAttachmentCandidate records one validated local attachment move.
// Target paths always use the canonical hashed task directory, even when the
// v5 source uses a legacy raw task-ID directory.
type V6RetainedAttachmentCandidate struct {
	ProjectID  string
	TaskID     string
	Filename   string
	SourcePath string
	Path       string
	Size       int64
	SHA256     string
}

// V6ObsoleteAttachmentCandidate is a legacy attachment directory that belongs
// to no retained task. Its bytes remain available in the immutable pre-v6
// snapshot, but activation removes the live directory instead of carrying an
// orphan across the schema boundary.
type V6ObsoleteAttachmentCandidate struct {
	ProjectID string
	Path      string
}

// V6WorkspaceConversionPlan is a filesystem-free publication candidate after
// construction: all source files have been read, every target byte is fixed,
// and retained attachment topology is represented by checked source/target
// pairs. Building the plan never writes or renames live data.
type V6WorkspaceConversionPlan struct {
	Domains             V6DomainConversionResult
	Projects            []V6ProjectDocumentCandidate
	Artifacts           []V6WorkspaceArtifactCandidate
	Attachments         []V6RetainedAttachmentCandidate
	ObsoleteAttachments []V6ObsoleteAttachmentCandidate
	TaskAudit           []format.V6TaskMigrationAuditEntry
}

type v6CandidateTask struct {
	ProjectID string
	Task      format.V6TaskRecord
	Next      bool
}

// DryRunV6WorkspaceConversion reads a complete schema-5 workspace and builds a
// deterministic, fully validated schema-6 candidate without publishing any
// write, rename, or cleanup operation.
//
//nolint:govet // Each validation stage keeps its error scope local.
func DryRunV6WorkspaceConversion(options V6WorkspaceDryRunOptions) (V6WorkspaceConversionPlan, error) {
	root, err := validateV5DryRunRoot(options.DataDir)
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}

	sourceCatalog, workspaceID, err := readV5CatalogCandidate(WorkspaceCatalogPath(root))
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	sourcePlanning, err := readV5PlanningCandidate(PlanningStatePath(root), workspaceID)
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	sourceMachine, machinePath, machineExists, err := readV5MachineCandidate(options, workspaceID)
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	if err := (V5DomainCandidates{
		Catalog: sourceCatalog, Planning: sourcePlanning, Machine: sourceMachine,
	}).Validate(); err != nil {
		return V6WorkspaceConversionPlan{}, fmt.Errorf("validate v5 source domains: %w", err)
	}

	targetCatalog, domainAudit, err := convertV6CatalogCandidate(root, sourceCatalog)
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	plan := V6WorkspaceConversionPlan{}
	allTasks := make(map[ProjectTaskLocator]v6CandidateTask)
	var orderedTasks []v6CandidateTask
	projectArtifacts := make([]V6WorkspaceArtifactCandidate, 0, len(sourceCatalog.Projects))

	for index, sourceProject := range sourceCatalog.Projects {
		targetProject := targetCatalog.Projects[index]
		candidate, artifact, converted, taskAudit, convertErr := convertV6ProjectCandidate(
			root, sourceProject, targetProject,
		)
		if convertErr != nil {
			return V6WorkspaceConversionPlan{}, convertErr
		}
		plan.Projects = append(plan.Projects, candidate)
		projectArtifacts = append(projectArtifacts, artifact)
		plan.TaskAudit = append(plan.TaskAudit, taskAudit...)
		for _, task := range converted {
			locator := ProjectTaskLocator{ProjectID: task.ProjectID, TaskID: task.Task.ID}
			if _, duplicate := allTasks[locator]; duplicate {
				return V6WorkspaceConversionPlan{}, fmt.Errorf(
					"v6 conversion repeats task %q in project %q", locator.TaskID, locator.ProjectID,
				)
			}
			allTasks[locator] = task
			orderedTasks = append(orderedTasks, task)
		}
	}

	targetPlanning, planningAudit := convertV6PlanningCandidate(sourcePlanning, allTasks, orderedTasks)
	domainAudit = append(domainAudit, planningAudit...)
	targetMachine, machineAudit := convertV6MachineCandidate(sourceMachine)
	domainAudit = append(domainAudit, machineAudit...)
	if !machineExists {
		domainAudit = append(domainAudit, V6DomainMigrationAuditEntry{
			Domain: "machine", Action: "substituted", Field: "domain",
			From: "absent", To: "dark theme",
			Reason: "absent v5 machine state was converted with the built-in dark theme",
		})
	}
	plan.Domains = V6DomainConversionResult{
		Domains: V6DomainCandidates{
			Catalog: targetCatalog, Planning: targetPlanning, Machine: targetMachine,
		},
		Audit: domainAudit,
	}

	domainArtifacts, err := encodeV6DomainArtifacts(root, machinePath, machineExists, plan.Domains.Domains)
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	plan.Artifacts = append(plan.Artifacts, domainArtifacts...)
	plan.Artifacts = append(plan.Artifacts, projectArtifacts...)

	noteArtifacts, err := collectV6NoteCandidates(root, sourceCatalog, targetCatalog)
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	plan.Artifacts = append(plan.Artifacts, noteArtifacts...)
	attachments, obsoleteAttachments, err := collectV6AttachmentCandidates(
		root, sourceCatalog, targetCatalog, plan.Projects,
	)
	if err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	plan.Attachments = attachments
	plan.ObsoleteAttachments = obsoleteAttachments

	if err := ValidateV6WorkspaceConversionPlan(plan); err != nil {
		return V6WorkspaceConversionPlan{}, err
	}
	return plan, nil
}

func readV5CatalogCandidate(path string) (V5WorkspaceCatalogCandidate, string, error) {
	data, err := readV5DryRunSource(path)
	if err != nil {
		return V5WorkspaceCatalogCandidate{}, "", err
	}
	var catalog V5WorkspaceCatalogCandidate
	workspaceID, err := decodeV5CandidateDomain(path, "workspace", "", data, &catalog)
	if err != nil {
		return V5WorkspaceCatalogCandidate{}, "", err
	}
	if err := catalog.Validate(); err != nil {
		return V5WorkspaceCatalogCandidate{}, "", err
	}
	if catalog.WorkspaceID != workspaceID {
		return V5WorkspaceCatalogCandidate{}, "", fmt.Errorf(
			"v5 workspace payload identity %q does not match envelope %q", catalog.WorkspaceID, workspaceID,
		)
	}
	return catalog, workspaceID, nil
}

func readV5PlanningCandidate(path, workspaceID string) (V5PlanningCandidate, error) {
	data, err := readV5DryRunSource(path)
	if err != nil {
		return V5PlanningCandidate{}, err
	}
	var planning V5PlanningCandidate
	if _, err := decodeV5CandidateDomain(path, "planning", workspaceID, data, &planning); err != nil {
		return V5PlanningCandidate{}, err
	}
	return planning, nil
}

func readV5MachineCandidate(
	options V6WorkspaceDryRunOptions,
	workspaceID string,
) (V5MachineCandidate, string, bool, error) {
	path := strings.TrimSpace(options.MachineStatePath)
	var err error
	if path == "" {
		path, err = MachineStatePath(workspaceID)
	}
	if err != nil {
		return V5MachineCandidate{}, "", false, err
	}
	path, err = filepath.Abs(filepath.Clean(path))
	if err != nil {
		return V5MachineCandidate{}, "", false, fmt.Errorf("resolve v6 dry-run machine path: %w", err)
	}
	data, found, err := readOptionalV5DryRunSource(path)
	if err != nil {
		return V5MachineCandidate{}, "", false, err
	}
	if !found {
		if strings.TrimSpace(options.MachineStatePath) != "" {
			return V5MachineCandidate{}, "", false, fmt.Errorf("inspect v5 machine source %s: %w", path, fs.ErrNotExist)
		}
		return V5MachineCandidate{Theme: "dark"}, path, false, nil
	}
	var machine V5MachineCandidate
	if _, err := decodeV5CandidateDomain(path, "machine", workspaceID, data, &machine); err != nil {
		return V5MachineCandidate{}, "", false, err
	}
	return machine, path, true, nil
}

func decodeV5CandidateDomain(
	path string,
	kind string,
	expectedWorkspaceID string,
	data []byte,
	out any,
) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope domainEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return "", fmt.Errorf("decode v5 %s envelope %s: %w", kind, path, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return "", fmt.Errorf("decode v5 %s envelope %s: %w", kind, path, err)
	}
	if envelope.SchemaVersion != CurrentDomainSchemaVersion || envelope.Kind != kind {
		return "", fmt.Errorf("v5 %s envelope %s has invalid schema or kind", kind, path)
	}
	if err := validateWorkspaceID(envelope.WorkspaceID); err != nil {
		return "", fmt.Errorf("v5 %s envelope %s: %w", kind, path, err)
	}
	if expectedWorkspaceID != "" && envelope.WorkspaceID != expectedWorkspaceID {
		return "", fmt.Errorf(
			"v5 %s envelope workspace %q does not match %q", kind, envelope.WorkspaceID, expectedWorkspaceID,
		)
	}
	payload := json.NewDecoder(bytes.NewReader(envelope.Data))
	payload.DisallowUnknownFields()
	if err := payload.Decode(out); err != nil {
		return "", fmt.Errorf("decode v5 %s payload %s: %w", kind, path, err)
	}
	if err := requireJSONEOF(payload); err != nil {
		return "", fmt.Errorf("decode v5 %s payload %s: %w", kind, path, err)
	}
	return envelope.WorkspaceID, nil
}

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
	return V6ProjectDocumentCandidate{
			ProjectID: targetProject.ID, SourcePath: sourcePath, Path: targetPath,
			SourceExists: found, Document: targetDocument,
		}, V6WorkspaceArtifactCandidate{
			SourcePath: sourcePath, Path: targetPath, Kind: "project",
			ProjectID: targetProject.ID, SourceExists: found, Data: []byte(content),
		}, convertedTasks, taskAudit, nil
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

func encodeV6DomainArtifacts(
	root string,
	machinePath string,
	machineExists bool,
	domains V6DomainCandidates,
) ([]V6WorkspaceArtifactCandidate, error) {
	specs := []struct {
		kind         string
		path         string
		sourceExists bool
		payload      any
	}{
		{kind: "workspace", path: WorkspaceCatalogPath(root), sourceExists: true, payload: domains.Catalog},
		{kind: "planning", path: PlanningStatePath(root), sourceExists: true, payload: domains.Planning},
		{kind: "machine", path: machinePath, sourceExists: machineExists, payload: domains.Machine},
	}
	artifacts := make([]V6WorkspaceArtifactCandidate, 0, len(specs))
	for _, spec := range specs {
		data, err := encodeV6DomainEnvelope(spec.kind, domains.Catalog.WorkspaceID, spec.payload)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, V6WorkspaceArtifactCandidate{
			SourcePath: spec.path, Path: spec.path, Kind: spec.kind,
			SourceExists: spec.sourceExists, Data: data,
		})
	}
	return artifacts, nil
}

func encodeV6DomainEnvelope(kind, workspaceID string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode v6 %s payload: %w", kind, err)
	}
	data, err := json.MarshalIndent(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          kind,
		Data:          raw,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode v6 %s envelope: %w", kind, err)
	}
	return data, nil
}

func collectV6NoteCandidates(
	root string,
	sourceCatalog V5WorkspaceCatalogCandidate,
	targetCatalog V6WorkspaceCatalogCandidate,
) ([]V6WorkspaceArtifactCandidate, error) {
	var candidates []V6WorkspaceArtifactCandidate
	global, err := collectV6NotesFromDir(
		storage.GlobalNotesDir(root), storage.GlobalNotesDir(root), "", "global-note",
	)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, global...)
	for index, sourceProject := range sourceCatalog.Projects {
		targetProject := targetCatalog.Projects[index]
		sourceDir := filepath.Join(root, "projects", sourceProject.Slug, "notes")
		targetDir := filepath.Join(root, "projects", targetProject.Slug, "notes")
		projectNotes, err := collectV6NotesFromDir(
			sourceDir, targetDir, targetProject.ID, "project-note",
		)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, projectNotes...)
	}
	return candidates, nil
}

func collectV6NotesFromDir(
	sourceDir string,
	targetDir string,
	projectID string,
	kind string,
) ([]V6WorkspaceArtifactCandidate, error) {
	info, err := os.Lstat(sourceDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect v5 note directory %s: %w", sourceDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("v5 note source is not a regular directory: %s", sourceDir)
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read v5 note directory %s: %w", sourceDir, err)
	}
	var candidates []V6WorkspaceArtifactCandidate
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("v6 conversion refuses symlink note source: %s", filepath.Join(sourceDir, entry.Name()))
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		sourcePath := filepath.Join(sourceDir, entry.Name())
		data, err := readV5DryRunSource(sourcePath)
		if err != nil {
			return nil, err
		}
		if err := storage.ValidateV5NoteContent(entry.Name(), data); err != nil {
			return nil, err
		}
		candidates = append(candidates, V6WorkspaceArtifactCandidate{
			SourcePath: sourcePath,
			Path:       filepath.Join(targetDir, entry.Name()),
			Kind:       kind, ProjectID: projectID, SourceExists: true,
			Data: append([]byte(nil), data...),
		})
	}
	return candidates, nil
}

func collectV6AttachmentCandidates(
	root string,
	sourceCatalog V5WorkspaceCatalogCandidate,
	targetCatalog V6WorkspaceCatalogCandidate,
	projects []V6ProjectDocumentCandidate,
) ([]V6RetainedAttachmentCandidate, []V6ObsoleteAttachmentCandidate, error) {
	documentByProject := make(map[string]storage.V6ProjectDocument, len(projects))
	for _, project := range projects {
		documentByProject[project.ProjectID] = project.Document
	}
	var candidates []V6RetainedAttachmentCandidate
	var obsolete []V6ObsoleteAttachmentCandidate
	for index, sourceProject := range sourceCatalog.Projects {
		targetProject := targetCatalog.Projects[index]
		projectCandidates, projectObsolete, err := collectV6ProjectAttachments(
			filepath.Join(root, "projects", sourceProject.Slug),
			filepath.Join(root, "projects", targetProject.Slug),
			targetProject.ID,
			documentByProject[targetProject.ID],
		)
		if err != nil {
			return nil, nil, err
		}
		candidates = append(candidates, projectCandidates...)
		obsolete = append(obsolete, projectObsolete...)
	}
	return candidates, obsolete, nil
}

//nolint:gocyclo // Attachment migration keeps identity collision and filesystem validation in one auditable pass.
func collectV6ProjectAttachments(
	sourceProjectDir string,
	targetProjectDir string,
	projectID string,
	document storage.V6ProjectDocument,
) ([]V6RetainedAttachmentCandidate, []V6ObsoleteAttachmentCandidate, error) {
	directoryTask := make(map[string]string, len(document.Tasks)*2)
	expectedFiles := make(map[string]map[string]bool, len(document.Tasks))
	for _, task := range document.Tasks {
		hashed := v6AttachmentTaskDirName(task.ID)
		if prior, collision := directoryTask[hashed]; collision && prior != task.ID {
			return nil, nil, fmt.Errorf("v6 attachment directory identity collision for task %q", task.ID)
		}
		directoryTask[hashed] = task.ID
		if storage.SafeChildName(task.ID) == nil {
			if prior, collision := directoryTask[task.ID]; collision && prior != task.ID {
				return nil, nil, fmt.Errorf("v6 attachment directory identity collision for task %q", task.ID)
			}
			directoryTask[task.ID] = task.ID
		}
		for _, attachment := range task.Attachments {
			if attachment.Kind != "file" {
				continue
			}
			if err := storage.SafeChildName(attachment.Filename); err != nil {
				return nil, nil, fmt.Errorf(
					"v6 task %q local attachment filename %q is unsafe: %w",
					task.ID, attachment.Filename, err,
				)
			}
			if expectedFiles[task.ID] == nil {
				expectedFiles[task.ID] = make(map[string]bool)
			}
			if expectedFiles[task.ID][attachment.Filename] {
				return nil, nil, fmt.Errorf(
					"v6 task %q repeats local attachment filename %q", task.ID, attachment.Filename,
				)
			}
			expectedFiles[task.ID][attachment.Filename] = true
		}
	}

	sourceRoot := filepath.Join(sourceProjectDir, ".attachments")
	info, err := os.Lstat(sourceRoot)
	if errors.Is(err, fs.ErrNotExist) {
		for taskID, files := range expectedFiles {
			if len(files) > 0 {
				return nil, nil, fmt.Errorf(
					"project %q task %q has local attachment metadata but no attachment directory",
					projectID, taskID,
				)
			}
		}
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("inspect v5 attachment root %s: %w", sourceRoot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil, fmt.Errorf("v5 attachment root is not a regular directory: %s", sourceRoot)
	}

	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("read v5 attachment root %s: %w", sourceRoot, err)
	}
	seenTaskDir := make(map[string]string)
	seenFiles := make(map[string]map[string]bool)
	var candidates []V6RetainedAttachmentCandidate
	var obsolete []V6ObsoleteAttachmentCandidate
	for _, entry := range entries {
		sourceTaskDir := filepath.Join(sourceRoot, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			return nil, nil, fmt.Errorf("v5 attachment task entry is not a regular directory: %s", sourceTaskDir)
		}
		taskID, exists := directoryTask[entry.Name()]
		if !exists {
			if err := validateV6ObsoleteAttachmentDir(sourceTaskDir); err != nil {
				return nil, nil, err
			}
			obsolete = append(obsolete, V6ObsoleteAttachmentCandidate{
				ProjectID: projectID,
				Path:      sourceTaskDir,
			})
			continue
		}
		if prior := seenTaskDir[taskID]; prior != "" {
			return nil, nil, fmt.Errorf(
				"project %q task %q has duplicate attachment directories %q and %q",
				projectID, taskID, prior, entry.Name(),
			)
		}
		seenTaskDir[taskID] = entry.Name()
		fileEntries, err := os.ReadDir(sourceTaskDir)
		if err != nil {
			return nil, nil, fmt.Errorf("read v5 task attachment directory %s: %w", sourceTaskDir, err)
		}
		seenFiles[taskID] = make(map[string]bool, len(fileEntries))
		for _, fileEntry := range fileEntries {
			sourcePath := filepath.Join(sourceTaskDir, fileEntry.Name())
			if fileEntry.Type()&os.ModeSymlink != 0 || fileEntry.IsDir() {
				return nil, nil, fmt.Errorf("v5 attachment source is not a regular file: %s", sourcePath)
			}
			if !expectedFiles[taskID][fileEntry.Name()] {
				return nil, nil, fmt.Errorf(
					"project %q task %q has orphan attachment file %q",
					projectID, taskID, fileEntry.Name(),
				)
			}
			data, err := readV5DryRunSource(sourcePath)
			if err != nil {
				return nil, nil, err
			}
			sum := sha256.Sum256(data)
			seenFiles[taskID][fileEntry.Name()] = true
			candidates = append(candidates, V6RetainedAttachmentCandidate{
				ProjectID:  projectID,
				TaskID:     taskID,
				Filename:   fileEntry.Name(),
				SourcePath: sourcePath,
				Path: filepath.Join(
					targetProjectDir, ".attachments", v6AttachmentTaskDirName(taskID), fileEntry.Name(),
				),
				Size:   int64(len(data)),
				SHA256: hex.EncodeToString(sum[:]),
			})
		}
	}
	for taskID, files := range expectedFiles {
		for filename := range files {
			if !seenFiles[taskID][filename] {
				return nil, nil, fmt.Errorf(
					"project %q task %q local attachment %q is missing",
					projectID, taskID, filename,
				)
			}
		}
	}
	return candidates, obsolete, nil
}

func validateV6ObsoleteAttachmentDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read obsolete v5 attachment directory %s: %w", path, err)
	}
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect obsolete v5 attachment %s: %w", entryPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("obsolete v5 attachment is not a regular file: %s", entryPath)
		}
	}
	return nil
}

func v6AttachmentTaskDirName(taskID string) string {
	sum := sha256.Sum256([]byte(taskID))
	return hex.EncodeToString(sum[:])
}

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
