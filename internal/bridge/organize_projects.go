package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/storage"
)

// impactView is what deleting a project would remove, with a digest of the
// project's stored files. A delete must echo the impact it showed, so one
// that another device's work changed in the meantime is refused rather than
// taking more than the person agreed to.
type impactView struct {
	Tasks       int    `json:"tasks"`
	Notes       int    `json:"notes"`
	Attachments int    `json:"attachments"`
	Digest      string `json:"digest"`
}

type projectDetail struct {
	Project projectView `json:"project"`
	Impact  impactView  `json:"impact"`
	// Comments are the project's own comments in stored order; an archived
	// project reports none.
	Comments []commentView `json:"comments"`
}

type projectPayload struct {
	ProjectID string `json:"project_id"`
}

// queryProject reads one project, archived ones included, with what deleting
// it would remove.
func (b *Bridge) queryProject(request Request) (any, error) {
	var payload projectPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.ProjectID == "" {
		return nil, newError(CodeInvalidRequest, "project_id is required")
	}
	return b.readProject(payload.ProjectID)
}

func (b *Bridge) readProject(projectID string) (projectDetail, error) {
	runtime := b.workspace.runtime
	project, impact, err := runtime.ProjectImpact(projectID)
	if err != nil {
		return projectDetail{}, err
	}
	digest, err := projectDigest(project.Path, impact)
	if err != nil {
		return projectDetail{}, err
	}
	comments, err := b.projectComments(projectID)
	if err != nil {
		return projectDetail{}, err
	}
	return projectDetail{
		Project:  viewProject(runtime.State(), project),
		Impact:   impactView{Tasks: impact.Tasks, Notes: impact.Notes, Attachments: impact.Attachments, Digest: digest},
		Comments: comments,
	}, nil
}

// projectDigest fingerprints the stored task Markdown beside the counts, so
// an edit that kept every count still changes it.
func projectDigest(projectPath string, impact engine.ProjectImpact) (string, error) {
	data, err := os.ReadFile(storage.TasksFilePath(projectPath))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%d|%d|%d", storage.RawArtifactRevision(data), impact.Tasks, impact.Notes, impact.Attachments))
	return hex.EncodeToString(sum[:]), nil
}

// projectCommandPayload is one project lifecycle action. edit keeps whatever
// it does not name; tags is always the complete set.
type projectCommandPayload struct {
	Action      string      `json:"action"`
	ProjectID   string      `json:"project_id,omitempty"`
	Name        *string     `json:"name,omitempty"`
	Description *string     `json:"description,omitempty"`
	Tags        *[]string   `json:"tags,omitempty"`
	Confirm     *impactView `json:"confirm,omitempty"`
}

type projectResult struct {
	Project   projectView `json:"project"`
	Committed committed   `json:"committed"`
}

// commandProject creates, edits, retags, parks, archives, restores, or deletes
// a project through the engine's one project lifecycle.
func (b *Bridge) commandProject(request Request) (any, error) {
	var payload projectCommandPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	runtime := b.workspace.runtime
	if payload.Action != "create" && payload.ProjectID == "" {
		return nil, newError(CodeInvalidRequest, "%s needs project_id", payload.Action)
	}
	var mutate func() (engine.ProjectChange, error)
	switch payload.Action {
	case "create":
		if payload.Name == nil {
			return nil, newError(CodeInvalidRequest, "create needs name")
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.CreateProject(*payload.Name) }
	case "edit":
		current, err := b.readProject(payload.ProjectID)
		if err != nil {
			return nil, err
		}
		edit := engine.ProjectEdit{Name: current.Project.Name, Description: current.Project.Description, Tags: current.Project.Tags}
		if payload.Name != nil {
			edit.Name = *payload.Name
		}
		if payload.Description != nil {
			edit.Description = *payload.Description
		}
		if payload.Tags != nil {
			edit.Tags = *payload.Tags
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.EditProject(payload.ProjectID, edit) }
	case "set_tags":
		if payload.Tags == nil {
			return nil, newError(CodeInvalidRequest, "set_tags needs tags")
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.SetProjectTags(payload.ProjectID, *payload.Tags) }
	case "archive":
		mutate = func() (engine.ProjectChange, error) { return runtime.ArchiveProject(payload.ProjectID) }
	case "unarchive":
		mutate = func() (engine.ProjectChange, error) { return runtime.UnarchiveProject(payload.ProjectID) }
	case "delete":
		if err := b.confirmDelete(payload); err != nil {
			return nil, err
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.DeleteProject(payload.ProjectID) }
	default:
		return nil, newError(CodeInvalidRequest, "action %q is not one of create, edit, set_tags, archive, unarchive, delete", payload.Action)
	}
	return b.commitProject(mutate)
}

// confirmDelete refuses a delete that does not echo the project's current
// impact exactly.
func (b *Bridge) confirmDelete(payload projectCommandPayload) error {
	if payload.Confirm == nil {
		return newError(CodeInvalidRequest, "delete needs confirm: the impact the confirmation showed")
	}
	current, err := b.readProject(payload.ProjectID)
	if err != nil {
		return err
	}
	if *payload.Confirm != current.Impact {
		return newError(CodeStaleRevision, "%s changed since the delete was confirmed; nothing was deleted", current.Project.Name).
			with("project_id", payload.ProjectID).with("impact", current.Impact)
	}
	return nil
}

func (b *Bridge) commitProject(mutate func() (engine.ProjectChange, error)) (projectResult, error) {
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return projectResult{}, err
	}
	before, err := currentRevisions(runtime)
	if err != nil {
		return projectResult{}, err
	}
	change, err := mutate()
	if err != nil {
		return projectResult{}, err
	}
	effect, err := b.reportMany(manyEffect{transactionID: change.TransactionID}, before)
	if err != nil {
		return projectResult{}, err
	}
	project := change.Project
	if stored := runtime.State().GetProject(project.ID); stored != nil {
		project = stored
	}
	return projectResult{Project: viewProject(runtime.State(), project), Committed: effect.Committed}, nil
}
