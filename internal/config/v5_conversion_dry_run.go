package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/storage"
)

// V5WorkspaceDryRunOptions identifies a v4 workspace and, when supplied, an
// explicit machine-state artifact. MachineStatePath is optional in production
// and primarily makes migration rehearsal independent of the host's home
// directory.
type V5WorkspaceDryRunOptions struct {
	DataDir          string
	MachineStatePath string
}

type V5WorkspaceArtifactCandidate struct {
	Path string
	Kind string
	Data []byte
}

// V5WorkspaceConversionPlan is a read-only migration result. Artifacts are the
// complete target bytes for the three retained domains and every retained note;
// RetiredDomainPaths and SnapshotOnlyProjectDirs are explicit cleanup guidance
// for the later snapshot-first activation transaction.
type V5WorkspaceConversionPlan struct {
	Domains                 V5DomainConversionResult
	Artifacts               []V5WorkspaceArtifactCandidate
	RetiredDomainPaths      []string
	SnapshotOnlyProjectDirs []string
	NoteAudit               []storage.V5NoteMigrationAuditEntry
}

// DryRunV5WorkspaceConversion reads and validates all configuration, lean
// project, machine, and note targets without writing live data. Repeating the
// call against unchanged source bytes produces an identical plan.
func DryRunV5WorkspaceConversion(options V5WorkspaceDryRunOptions) (V5WorkspaceConversionPlan, error) {
	root, err := validateV5DryRunRoot(options.DataDir)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}

	sourcePaths := struct {
		workspace  string
		planning   string
		settings   string
		automation string
	}{
		workspace:  WorkspaceCatalogPath(root),
		planning:   PlanningStatePath(root),
		settings:   SettingsPath(root),
		automation: AutomationPath(root),
	}
	workspaceData, err := readV5DryRunSource(sourcePaths.workspace)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	workspaceEnvelope, err := decodeLegacyV4DomainEnvelope("workspace", workspaceData)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	planningData, err := readV5DryRunSource(sourcePaths.planning)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	settingsData, settingsFound, err := readOptionalV5DryRunSource(sourcePaths.settings)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	automationData, automationFound, err := readOptionalV5DryRunSource(sourcePaths.automation)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	var sourceAudit []V5DomainMigrationAuditEntry
	if !settingsFound {
		settingsData, err = emptyLegacyV4DomainArtifact("settings", workspaceEnvelope.WorkspaceID)
		if err != nil {
			return V5WorkspaceConversionPlan{}, err
		}
		sourceAudit = append(sourceAudit, absentV5DomainSourceAudit("settings"))
	}
	if !automationFound {
		automationData, err = emptyLegacyV4DomainArtifact("automation", workspaceEnvelope.WorkspaceID)
		if err != nil {
			return V5WorkspaceConversionPlan{}, err
		}
		sourceAudit = append(sourceAudit, absentV5DomainSourceAudit("automation"))
	}

	machinePath := options.MachineStatePath
	if strings.TrimSpace(machinePath) == "" {
		machinePath, err = MachineStatePath(workspaceEnvelope.WorkspaceID)
	}
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	machinePath, err = filepath.Abs(filepath.Clean(machinePath))
	if err != nil {
		return V5WorkspaceConversionPlan{}, fmt.Errorf("resolve v5 dry-run machine state path: %w", err)
	}

	machineData, machineSourcePath, err := readV5MachineSource(
		root,
		machinePath,
		strings.TrimSpace(options.MachineStatePath) != "",
	)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	switch {
	case machineSourcePath == "":
		machineData, err = emptyLegacyV4DomainArtifact("machine", workspaceEnvelope.WorkspaceID)
		if err != nil {
			return V5WorkspaceConversionPlan{}, err
		}
		sourceAudit = append(sourceAudit, absentV5DomainSourceAudit("machine"))
	case machineSourcePath != machinePath:
		sourceAudit = append(sourceAudit, V5DomainMigrationAuditEntry{
			Domain: "machine",
			Action: "substituted",
			Field:  "domain",
			From:   machineSourcePath,
			To:     machinePath,
			Reason: "legacy machine state was converted into the canonical workspace-keyed domain",
		})
	}

	legacy, err := DecodeLegacyV4Domains(LegacyV4DomainArtifacts{
		Workspace:  workspaceData,
		Planning:   planningData,
		Settings:   settingsData,
		Automation: automationData,
		Machine:    machineData,
	})
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	converted, err := ConvertLegacyV4Domains(legacy)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	converted.Audit = append(converted.Audit, sourceAudit...)

	workspaceTarget, err := encodeV5DomainEnvelope("workspace", legacy.WorkspaceID, converted.Domains.Catalog)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	planningTarget, err := encodeV5DomainEnvelope("planning", legacy.WorkspaceID, converted.Domains.Planning)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	machineTarget, err := encodeV5DomainEnvelope("machine", legacy.WorkspaceID, converted.Domains.Machine)
	if err != nil {
		return V5WorkspaceConversionPlan{}, err
	}

	plan := V5WorkspaceConversionPlan{
		Domains: converted,
		Artifacts: []V5WorkspaceArtifactCandidate{
			{Path: sourcePaths.workspace, Kind: "workspace", Data: workspaceTarget},
			{Path: sourcePaths.planning, Kind: "planning", Data: planningTarget},
			{Path: machinePath, Kind: "machine", Data: machineTarget},
		},
		RetiredDomainPaths: []string{sourcePaths.settings, sourcePaths.automation},
	}
	for _, slug := range converted.RemovedProjectSlugs {
		plan.SnapshotOnlyProjectDirs = append(
			plan.SnapshotOnlyProjectDirs,
			filepath.Join(root, "projects", slug),
		)
	}

	if err := appendV5NoteCandidates(&plan, root, converted.Domains.Catalog); err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	if err := validateV5WorkspaceArtifactCandidates(plan, legacy.WorkspaceID); err != nil {
		return V5WorkspaceConversionPlan{}, err
	}
	return plan, nil
}

func validateV5DryRunRoot(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("v5 conversion dry run requires a workspace data dir")
	}
	root, err := filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return "", fmt.Errorf("resolve v5 conversion dry-run root: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect v5 conversion dry-run root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("v5 conversion dry run refuses a symlink workspace root: %s", root)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("v5 conversion dry-run root is not a directory: %s", root)
	}
	return root, nil
}

func readV5DryRunSource(path string) ([]byte, error) {
	data, found, err := readOptionalV5DryRunSource(path)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("inspect v5 conversion source %s: %w", path, fs.ErrNotExist)
	}
	return data, nil
}

func readOptionalV5DryRunSource(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect v5 conversion source %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("v5 conversion dry run refuses symlink source: %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("v5 conversion source is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read v5 conversion source %s: %w", path, err)
	}
	return data, true, nil
}

func emptyLegacyV4DomainArtifact(kind, workspaceID string) ([]byte, error) {
	data, err := json.Marshal(legacyV4DomainEnvelope{
		SchemaVersion: legacyV4DomainEnvelopeSchema,
		WorkspaceID:   workspaceID,
		Kind:          kind,
		Data:          json.RawMessage("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("encode empty legacy v4 %s domain: %w", kind, err)
	}
	return data, nil
}

func absentV5DomainSourceAudit(domain string) V5DomainMigrationAuditEntry {
	return V5DomainMigrationAuditEntry{
		Domain: domain,
		Action: "substituted",
		Field:  "domain",
		From:   "absent",
		To:     "empty v4 " + domain + " state",
		Reason: "optional legacy domain was absent and migrated with safe defaults",
	}
}

func readV5MachineSource(
	dataDir string,
	targetPath string,
	explicitPath bool,
) ([]byte, string, error) {
	candidates := []string{targetPath}
	if !explicitPath {
		candidates = append(candidates, legacyMachineStatePaths(dataDir)...)
	}
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		path, err := filepath.Abs(filepath.Clean(candidate))
		if err != nil {
			return nil, "", fmt.Errorf("resolve v5 dry-run machine state source %s: %w", candidate, err)
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		data, found, err := readOptionalV5DryRunSource(path)
		if err != nil {
			return nil, "", err
		}
		if found {
			return data, path, nil
		}
	}
	return nil, "", nil
}

func appendV5NoteCandidates(
	plan *V5WorkspaceConversionPlan,
	root string,
	catalog V5WorkspaceCatalogCandidate,
) error {
	if plan == nil {
		return fmt.Errorf("v5 workspace conversion plan is required")
	}
	if err := appendV5NotesFromDir(plan, storage.GlobalNotesDir(root), "global-note"); err != nil {
		return err
	}
	for _, project := range catalog.Projects {
		dir := filepath.Join(root, "projects", project.Slug, "notes")
		if err := appendV5NotesFromDir(plan, dir, "project-note:"+project.ID); err != nil {
			return err
		}
	}
	return nil
}

func appendV5NotesFromDir(plan *V5WorkspaceConversionPlan, dir, kind string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect v5 note source directory %s: %w", dir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("v5 conversion dry run refuses symlink note directory: %s", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("v5 note source is not a directory: %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read v5 note source directory %s: %w", dir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := readV5DryRunSource(path)
		if err != nil {
			return err
		}
		legacy, err := storage.DecodeLegacyV4Note(entry.Name(), data)
		if err != nil {
			return err
		}
		converted, err := storage.ConvertLegacyV4Note(legacy)
		if err != nil {
			return err
		}
		plan.Artifacts = append(plan.Artifacts, V5WorkspaceArtifactCandidate{
			Path: path,
			Kind: kind,
			Data: append([]byte(nil), converted.Data...),
		})
		plan.NoteAudit = append(plan.NoteAudit, converted.Audit...)
	}
	return nil
}

func validateV5WorkspaceArtifactCandidates(plan V5WorkspaceConversionPlan, workspaceID string) error {
	if err := plan.Domains.Domains.Validate(); err != nil {
		return err
	}
	paths := make(map[string]bool, len(plan.Artifacts))
	domainKinds := make(map[string]bool, 3)
	for _, artifact := range plan.Artifacts {
		cleanPath := filepath.Clean(artifact.Path)
		if paths[cleanPath] {
			return fmt.Errorf("v5 conversion produced duplicate target path %s", cleanPath)
		}
		paths[cleanPath] = true
		switch artifact.Kind {
		case "workspace", "planning", "machine":
			if domainKinds[artifact.Kind] {
				return fmt.Errorf("v5 conversion produced duplicate %s domain", artifact.Kind)
			}
			domainKinds[artifact.Kind] = true
			if err := validateV5DomainCandidateBytes(artifact.Kind, workspaceID, artifact.Data, plan.Domains.Domains); err != nil {
				return err
			}
		default:
			if !strings.HasPrefix(artifact.Kind, "global-note") &&
				!strings.HasPrefix(artifact.Kind, "project-note:") {
				return fmt.Errorf("v5 conversion produced unknown target kind %q", artifact.Kind)
			}
			if err := storage.ValidateV5NoteContent(filepath.Base(cleanPath), artifact.Data); err != nil {
				return err
			}
		}
	}
	for _, kind := range []string{"workspace", "planning", "machine"} {
		if !domainKinds[kind] {
			return fmt.Errorf("v5 conversion did not produce a %s domain", kind)
		}
	}
	return nil
}

func validateV5DomainCandidateBytes(
	kind string,
	workspaceID string,
	data []byte,
	expected V5DomainCandidates,
) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var env legacyV4DomainEnvelope
	if err := decoder.Decode(&env); err != nil {
		return fmt.Errorf("validate v5 %s envelope: %w", kind, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return fmt.Errorf("validate v5 %s envelope: %w", kind, err)
	}
	if env.SchemaVersion != CurrentDomainSchemaVersion ||
		env.WorkspaceID != workspaceID ||
		env.Kind != kind {
		return fmt.Errorf("validate v5 %s envelope: identity mismatch", kind)
	}
	switch kind {
	case "workspace":
		var candidate V5WorkspaceCatalogCandidate
		if err := json.Unmarshal(env.Data, &candidate); err != nil {
			return fmt.Errorf("validate v5 workspace payload: %w", err)
		}
		if err := candidate.Validate(); err != nil {
			return err
		}
	case "planning":
		var candidate V5PlanningCandidate
		if err := json.Unmarshal(env.Data, &candidate); err != nil {
			return fmt.Errorf("validate v5 planning payload: %w", err)
		}
		if err := candidate.Validate(expected.Catalog); err != nil {
			return err
		}
	case "machine":
		var candidate V5MachineCandidate
		if err := json.Unmarshal(env.Data, &candidate); err != nil {
			return fmt.Errorf("validate v5 machine payload: %w", err)
		}
		if err := candidate.Validate(expected.Catalog); err != nil {
			return err
		}
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("trailing JSON value")
	}
	return err
}
