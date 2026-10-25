package projectops

import (
	"fmt"
	"slices"
	"sort"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/filterops"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/valuepicker"

	tea "charm.land/bubbletea/v2"
)

// The vocabulary actions edit one project tag across every project that carries
// it. They are reachable only from a tag row inside the F# picker, which is the
// one surface that names a tag rather than a project.
const (
	OperationProjectTagRenameAll operation.OperationID = "tui.project-tag.rename-all"
	OperationProjectTagRemoveAll operation.OperationID = "tui.project-tag.remove-all"
	OperationProjectTagParkAll   operation.OperationID = "tui.project-tag.park-all"
	OperationProjectTagUnparkAll operation.OperationID = "tui.project-tag.unpark-all"
)

const projectTagActionCategory listsurface.ListCategoryID = "project-tag-actions"

var projectTagActionCategories = []listsurface.ListCategory{{ID: projectTagActionCategory, Label: "Tag"}}

// projectTagVocabularyOperations is the closed, ordered sheet.
var projectTagVocabularyOperations = []operation.OperationID{
	OperationProjectTagRenameAll,
	OperationProjectTagRemoveAll,
	OperationProjectTagParkAll,
	OperationProjectTagUnparkAll,
}

var projectTagVocabularyActions = map[operation.OperationID]projecthub.ProjectBulkTagAction{
	OperationProjectTagRenameAll: projecthub.ProjectBulkTagRename,
	OperationProjectTagRemoveAll: projecthub.ProjectBulkTagRemove,
	OperationProjectTagParkAll:   projecthub.ProjectBulkTagPark,
	OperationProjectTagUnparkAll: projecthub.ProjectBulkTagUnpark,
}

func projectTagVocabularyDescriptors() []operation.OperationDescriptor {
	descriptor := func(id operation.OperationID, label, description string) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Category:             operation.OperationCategoryProject,
			TargetSource:         operation.OperationTargetSelectedEntities,
			TargetCardinality:    operation.TargetCardinalityOneOrMore,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        projecthub.ProjectBulkTagsApplicable,
			Risk:                 operation.OperationRiskMutation,
		}
	}
	return []operation.OperationDescriptor{
		operation.WithValueProvider(
			descriptor(OperationProjectTagRenameAll, "Rename Everywhere", "Rename this tag on every project carrying it"),
			operation.ValueProviderTagName,
		),
		operation.WithValueProvider(
			descriptor(OperationProjectTagRemoveAll, "Remove Everywhere", "Remove this tag from every project carrying it"),
			operation.ValueProviderProjectTagDestination,
		),
		descriptor(OperationProjectTagParkAll, "Park All Members", "Park every project carrying this tag"),
		descriptor(OperationProjectTagUnparkAll, "Unpark All Members", "Unpark every project carrying this tag"),
	}
}

func (c *Controller) RegisterProjectTagVocabularyOperations(shell Shell) error {
	if shell.Registry() == nil || shell.Executor() == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := shell.Executor().RegisterValueProvider(
		operation.ValueProviderProjectTagDestination,
		func(invocation operation.OperationInvocation) operation.OperationValueProviderResult {
			return c.projectTagDestinationValue(shell, invocation)
		},
	); err != nil {
		return err
	}
	for _, descriptor := range projectTagVocabularyDescriptors() {
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			BulkHandler: func(invocation operation.OperationInvocation, targets []operation.OperationTarget) operation.OperationHandlerResult {
				return c.ExecuteProjectTagVocabulary(shell, invocation, targets)
			},
			// The bulk tag pipeline already owns the park prompt, the single
			// catalog transaction, and its undo record.
			DelegateConfirmation: true,
			DelegateTransaction:  true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteProjectTagVocabulary turns one sheet activation into the existing bulk
// tag request. The subject tag comes from the shell rather than the frozen sheet
// value because a nested value step overwrites that value on its way back.
func (c *Controller) ExecuteProjectTagVocabulary(
	shell Shell,
	invocation operation.OperationInvocation,
	targets []operation.OperationTarget,
) operation.OperationHandlerResult {
	if c.tagActionSubject == "" {
		return operation.OperationHandlerResult{Err: fmt.Errorf("no project tag is selected")}
	}
	message := projecthub.ProjectBulkTagsMsg{
		ProjectIDs: make([]string, 0, len(targets)),
		Action:     projectTagVocabularyActions[invocation.Descriptor.ID],
		Tag:        c.tagActionSubject,
	}
	for _, target := range targets {
		if target.Entity.Kind != operation.EntityKindProject || target.ProjectID == "" {
			return operation.OperationHandlerResult{Err: fmt.Errorf(
				"%s needs project targets, got %s",
				invocation.Descriptor.Label,
				target.Entity,
			)}
		}
		message.ProjectIDs = append(message.ProjectIDs, target.ProjectID)
	}
	if invocation.Descriptor.ValueProviderID != "" {
		replacement, ok := invocation.Value.(string)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf(
				"%s needs a tag name",
				invocation.Descriptor.Label,
			)}
		}
		message.Replacement = replacement
	}
	c.tagActionSubject = ""
	return operation.OperationHandlerResult{Message: message}
}

// OpenProjectTagVocabularyActions freezes the tag's member projects into the
// sheet context, so the eventual edit names the same projects the sheet counted
// even though the picker beneath it has already closed.
func (c *Controller) OpenProjectTagVocabularyActions(shell Shell, tag string) tea.Cmd {
	members := c.TagMembers(shell, tag)
	if len(members) == 0 {
		return status.ErrorCmd("no active project carries #%s", tag)
	}
	ctx := shell.OperationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	ctx.Selection.EntityIdentities = make([]operation.EntityIdentity, 0, len(members))
	for _, member := range members {
		ctx.Selection.EntityIdentities = append(
			ctx.Selection.EntityIdentities,
			operation.ProjectEntity(member.ID),
		)
	}

	sheet, err := listsurface.NewContextActionSheetModel(
		shell.Registry(),
		shell.Executor(),
		ctx,
		projectTagVocabularyOperations,
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostOutliner,
			Completion:   listsurface.ContextActionCompleteCloseSheet,
			Title:        "Tag Actions",
			Subtitle:     "#" + tag,
			ItemIDPrefix: "project-tag-action:",
			Categories:   projectTagActionCategories,
			CategoryFor: func(operation.OperationDescriptor) listsurface.ListCategoryID {
				return projectTagActionCategory
			},
			SecondaryFor: operation.ActionSecondaryText,
			Bulk:         true,
			Targets:      len(members),
			EmptyState:   listsurface.ListEmptyState{Title: "No tag actions available."},
			NoMatchState: listsurface.ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
		},
	)
	if err != nil {
		return status.ErrorCmd("%v", err)
	}
	c.tagActionSubject = tag
	shell.MountActionSheet(sheet)
	return nil
}

// TagMembers is the tag's editable membership. Archived projects refuse
// tag edits, so they are excluded here rather than failing the whole request
// once the transaction is already being planned.
func (c *Controller) TagMembers(shell Shell, tag string) []*models.Project {
	members := make([]*models.Project, 0)
	for _, project := range shell.AvailableProjects() {
		if project != nil && slices.Contains(project.Tags, tag) {
			members = append(members, project)
		}
	}
	return members
}

// projectTagDestinationValue skips the destination prompt unless removing the
// tag would leave a member carrying no tags at all. Only a stranded project has
// a question to answer; everything else keeps the tags it already has.
func (c *Controller) projectTagDestinationValue(shell Shell, _ operation.OperationInvocation) operation.OperationValueProviderResult {
	if len(c.strandedProjectTagMembers(shell)) == 0 {
		return operation.OperationValueProviderResult{Value: "", HasValue: true}
	}
	return operation.OperationValueProviderResult{}
}

func (c *Controller) strandedProjectTagMembers(shell Shell) []*models.Project {
	tag := c.tagActionSubject
	if tag == "" {
		return nil
	}
	stranded := make([]*models.Project, 0)
	for _, member := range c.TagMembers(shell, tag) {
		if len(member.Tags) == 1 {
			stranded = append(stranded, member)
		}
	}
	return stranded
}

// NewProjectTagDestinationPicker asks which tag the stranded members inherit.
// Leaving them untagged is a legitimate answer, so it is a row rather than a
// cancellation.
func (c *Controller) NewProjectTagDestinationPicker(
	shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	subject := c.tagActionSubject
	if subject == "" {
		return nil, fmt.Errorf("no project tag is selected")
	}
	counts := shell.ProjectTagVocabularyCounts()
	names := make([]string, 0, len(counts))
	for name := range counts {
		if name != subject && counts[name] > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	items := []listsurface.ListItem{{
		ID:            "tag-destination:",
		CategoryID:    valuepicker.TagDescriptiveCategory,
		PrimaryText:   "Leave untagged",
		SecondaryText: "members keep no tags",
		Aliases:       []string{"none", "untagged"},
		Data:          "",
	}}
	for _, name := range names {
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("tag-destination:" + name),
			CategoryID:    valuepicker.TagDescriptiveCategory,
			PrimaryText:   name,
			SecondaryText: filterops.ProjectCountPhrase(counts[name]),
			Aliases:       []string{name, "#" + name},
			Data:          name,
		})
	}

	stranded := c.strandedProjectTagMembers(shell)
	subtitle := fmt.Sprintf(
		"%s would lose every tag",
		filterops.ProjectCountPhrase(len(stranded)),
	)
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderProjectTagDestination,
		"Remove #"+subject+" everywhere",
		subtitle,
		listsurface.ListSource{
			Items:      items,
			Categories: []listsurface.ListCategory{{ID: valuepicker.TagDescriptiveCategory, Label: "Destination"}},
		},
		false,
		nil,
	)
}
