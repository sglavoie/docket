package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

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
