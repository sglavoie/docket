package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/docket/internal/storage"
)

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
