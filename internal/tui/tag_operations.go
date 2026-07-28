package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// Tag operation IDs. A tag identity is either the workspace aggregate the
// Finder lists or one project's catalog entry, and the two scopes reach
// different mutation seams.
const (
	operationTagFilter          OperationID = "tui.tag.filter"
	operationTagManage          OperationID = "tui.tag.manage"
	operationTagPin             OperationID = "tui.tag.pin"
	operationTagRenameWorkspace OperationID = "tui.tag.rename-workspace"
	operationTagRenameProject   OperationID = "tui.tag.rename-project"
	operationTagDeleteProject   OperationID = "tui.tag.delete-project"
)

// ApplyTagFilterMsg applies exactly one tag to the filter of the view the
// selection originated from, matching the other tag-filter entry points.
type ApplyTagFilterMsg struct {
	Source PresentationID
	Tag    string
}

// tagOperationDescriptors is the authoritative catalog of tag-targeted
// operations. Deletion is deliberately absent for workspace tags: an aggregate
// row spans every project that uses the tag, so there is no single catalog the
// deletion could name.
func tagOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		description string,
		risk OperationRisk,
		applicability OperationApplicability,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Aliases:           aliases,
			Category:          OperationCategoryTag,
			Placement:         PlacementFinderOperation,
			TargetSource:      OperationTargetCurrentEntity,
			TargetCardinality: TargetCardinalityOne,
			Applicability:     applicability,
			Risk:              risk,
		}
	}
	return []OperationDescriptor{
		descriptor(operationTagFilter, "Filter by tag", "Filter the current view by this tag", OperationRiskReadOnly, tagFilterOperationApplicable, "tag filter"),
		descriptor(operationTagManage, "Manage tags", "Open the tag manager for this tag's scope", OperationRiskReadOnly, tagOperationApplicable),
		descriptor(operationTagPin, "Toggle tag pin", "Pin or unpin this tag", OperationRiskMutation, tagOperationApplicable, "pin tag", "unpin tag"),
		withValueProvider(
			descriptor(operationTagRenameWorkspace, "Rename tag everywhere", "Rename this tag across every project that uses it", OperationRiskMutation, workspaceTagOperationApplicable),
			ValueProviderTagName,
		),
		withValueProvider(
			descriptor(operationTagRenameProject, "Rename project tag", "Rename this tag inside its project", OperationRiskMutation, projectTagOperationApplicable),
			ValueProviderTagName,
		),
		descriptor(operationTagDeleteProject, "Delete project tag", "Remove this tag from its project catalog, tasks, and pins", OperationRiskDestructive, projectTagOperationApplicable),
	}
}

func tagOperationApplicable(ctx CommandContext) bool {
	return currentEntityIdentity(ctx).Kind == EntityKindTag
}

// tagFilterOperationApplicable also requires a destination that owns a tag
// filter, so the action is never advertised where applying it would do nothing.
func tagFilterOperationApplicable(ctx CommandContext) bool {
	return tagOperationApplicable(ctx) && tagFilterAppliesTo(ctx.View)
}

func workspaceTagOperationApplicable(ctx CommandContext) bool {
	identity := currentEntityIdentity(ctx)
	return identity.Kind == EntityKindTag && identity.Scope == EntityScopeGlobal
}

func projectTagOperationApplicable(ctx CommandContext) bool {
	identity := currentEntityIdentity(ctx)
	return identity.Kind == EntityKindTag && identity.Scope == EntityScopeProject
}

func (m *AppModel) registerTagOperations() error {
	if m == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		ValueProviderTagName,
		pendingTextValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range tagOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			SingleHandler:       m.executeTagOperation,
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// resolvedTag is one tag confirmed to still exist in the scope its identity
// names. Project is nil for a workspace aggregate tag.
type resolvedTag struct {
	Name    string
	Project *models.Project
}

// resolveTagTarget rechecks the tag against live project data so a tag renamed
// or removed since the row was built fails instead of mutating a stale name.
func (m *AppModel) resolveTagTarget(identity EntityIdentity) (resolvedTag, error) {
	name := models.NormalizeTag(identity.Name)
	if name == "" {
		return resolvedTag{}, fmt.Errorf("tag target has no name")
	}
	switch identity.Scope {
	case EntityScopeGlobal:
		for _, tag := range finderTagsFromProjects(m.runtime, m.activeProjects()) {
			if tag.Name == name {
				return resolvedTag{Name: name}, nil
			}
		}
		return resolvedTag{}, fmt.Errorf("tag %s is no longer used in this workspace", name)
	case EntityScopeProject:
		project := m.projectByID(identity.ProjectID)
		if project == nil {
			return resolvedTag{}, fmt.Errorf("project %s is no longer available", identity.ProjectID)
		}
		for _, existing := range CollectTagNamesForProject(project) {
			if existing == name {
				return resolvedTag{Name: name, Project: project}, nil
			}
		}
		return resolvedTag{}, fmt.Errorf("tag %s is no longer part of %s", name, project.Name)
	default:
		return resolvedTag{}, fmt.Errorf("tag %s names no scope", identity)
	}
}

//nolint:exhaustive // Only tag operations are registered with this handler.
func (m *AppModel) executeTagOperation(
	invocation OperationInvocation,
	target OperationTarget,
) OperationHandlerResult {
	if target.Entity.Kind != EntityKindTag {
		return OperationHandlerResult{
			Err: fmt.Errorf("%s needs a tag target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	resolved, err := m.resolveTagTarget(target.Entity)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	coordinator := m.secondaryFeatures.Tag

	switch invocation.Descriptor.ID {
	case operationTagFilter:
		return OperationHandlerResult{Message: ApplyTagFilterMsg{
			Source: invocation.Context.View,
			Tag:    resolved.Name,
		}}

	case operationTagManage:
		if resolved.Project == nil {
			return OperationHandlerResult{Message: ShowGlobalTagManagerMsg{}}
		}
		return OperationHandlerResult{Message: ShowTagManagerMsg{Project: resolved.Project}}

	case operationTagPin:
		if resolved.Project == nil {
			return OperationHandlerResult{Message: OperationCommandResultMsg{
				Cmd: toggleGlobalPinnedTagCmd(m.runtime, resolved.Name),
			}}
		}
		return OperationHandlerResult{Message: OperationCommandResultMsg{
			Cmd: toggleProjectPinnedTagCmd(coordinator, resolved.Project.ID, resolved.Name),
		}}

	case operationTagRenameWorkspace:
		newName, err := tagOperationNewName(invocation, resolved.Name)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return OperationHandlerResult{Message: OperationCommandResultMsg{
			Cmd: renameWorkspaceTagCmd(coordinator, resolved.Name, newName),
		}}

	case operationTagRenameProject:
		newName, err := tagOperationNewName(invocation, resolved.Name)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return OperationHandlerResult{Message: OperationCommandResultMsg{
			Cmd: renameProjectTagCmd(coordinator, resolved.Project.ID, resolved.Name, newName),
		}}

	case operationTagDeleteProject:
		return OperationHandlerResult{Message: OperationCommandResultMsg{
			Cmd: removeProjectTagCmd(coordinator, resolved.Project.ID, resolved.Name),
		}}

	default:
		return OperationHandlerResult{Err: fmt.Errorf("%w: %s", ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

func tagOperationNewName(invocation OperationInvocation, current string) (string, error) {
	name := models.NormalizeTag(strings.TrimSpace(operationStringValue(invocation)))
	if name == "" {
		return "", fmt.Errorf("%s requires a new tag name", invocation.Descriptor.Label)
	}
	if name == current {
		return "", fmt.Errorf("%s is already named %s", current, name)
	}
	return name, nil
}

// renameProjectTagCmd renames a tag across one project's catalog, task usages,
// and pins as a single transaction.
func renameProjectTagCmd(coordinator TagCoordinator, projectID, oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		newName = models.NormalizeTag(newName)
		if newName == "" || newName == oldName {
			return nil
		}
		res, err := coordinator.RenameCatalog(projectID, oldName, newName)
		if err != nil {
			return tagManagerError(err)
		}
		if !res.Changed {
			return nil
		}
		return TagRenamedMsg{OldName: oldName, NewName: res.Tag}
	}
}

// removeProjectTagCmd deletes a tag from one project's catalog, task usages,
// and pins as a single transaction.
func removeProjectTagCmd(coordinator TagCoordinator, projectID, tagName string) tea.Cmd {
	return func() tea.Msg {
		res, err := coordinator.RemoveCatalog(projectID, tagName)
		if err != nil {
			return tagManagerError(err)
		}
		return TagRemovedMsg{Name: res.Tag}
	}
}

func addProjectTagCmd(coordinator TagCoordinator, projectID, tagName string) tea.Cmd {
	return func() tea.Msg {
		res, err := coordinator.AddCatalog(projectID, tagName)
		if err != nil {
			return tagManagerError(err)
		}
		return TagAddedMsg{Name: res.Tag}
	}
}

func toggleProjectPinnedTagCmd(coordinator TagCoordinator, projectID, tagName string) tea.Cmd {
	return func() tea.Msg {
		res, err := coordinator.TogglePinned(projectID, tagName)
		if err != nil {
			return tagManagerError(err)
		}
		return TagPinnedMsg{Name: res.Tag, Pinned: res.Pinned}
	}
}

// renameWorkspaceTagCmd renames a tag across every project that uses it as one
// grouped transaction.
func renameWorkspaceTagCmd(coordinator TagCoordinator, oldName, newName string) tea.Cmd {
	oldName = models.NormalizeTag(oldName)
	newName = models.NormalizeTag(newName)
	if newName == "" || newName == oldName {
		return nil
	}
	return func() tea.Msg {
		res, err := coordinator.RenameWorkspace(oldName, newName)
		if err != nil {
			return GlobalTagManagerErrorMsg{Err: globalTagRenameError(err)}
		}
		return GlobalTagRenamedMsg{
			OldName:      res.OldTag,
			NewName:      res.NewTag,
			ProjectCount: res.ProjectCount,
			TaskCount:    res.TaskCount,
		}
	}
}

func toggleGlobalPinnedTagCmd(runtime *config.RuntimeState, tagName string) tea.Cmd {
	return func() tea.Msg {
		if runtime == nil {
			return GlobalTagManagerErrorMsg{Err: fmt.Errorf("config is unavailable")}
		}
		tagName = models.NormalizeTag(tagName)
		var pinned bool
		var toggleErr error
		if err := commitSettingsToRuntime(runtime, func(s *config.Settings) {
			pinned, toggleErr = s.ToggleGlobalPinnedTag(tagName)
		}); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}
		if toggleErr != nil {
			return GlobalTagManagerErrorMsg{Err: toggleErr}
		}
		return GlobalTagPinnedMsg{Name: tagName, Pinned: pinned}
	}
}
