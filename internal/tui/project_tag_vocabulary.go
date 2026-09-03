package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"

	tea "charm.land/bubbletea/v2"
)

// The vocabulary actions edit one project tag across every project that carries
// it. They are reachable only from a tag row inside the F# picker, which is the
// one surface that names a tag rather than a project.
const (
	operationProjectTagRenameAll OperationID = "tui.project-tag.rename-all"
	operationProjectTagRemoveAll OperationID = "tui.project-tag.remove-all"
	operationProjectTagParkAll   OperationID = "tui.project-tag.park-all"
	operationProjectTagUnparkAll OperationID = "tui.project-tag.unpark-all"
)

const projectTagActionCategory ListCategoryID = "project-tag-actions"

var projectTagActionCategories = []ListCategory{{ID: projectTagActionCategory, Label: "Tag"}}

// projectTagVocabularyOperations is the closed, ordered sheet.
var projectTagVocabularyOperations = []OperationID{
	operationProjectTagRenameAll,
	operationProjectTagRemoveAll,
	operationProjectTagParkAll,
	operationProjectTagUnparkAll,
}

var projectTagVocabularyActions = map[OperationID]ProjectBulkTagAction{
	operationProjectTagRenameAll: ProjectBulkTagRename,
	operationProjectTagRemoveAll: ProjectBulkTagRemove,
	operationProjectTagParkAll:   ProjectBulkTagPark,
	operationProjectTagUnparkAll: ProjectBulkTagUnpark,
}

// ProjectTagVocabularyActionsMsg opens the vocabulary sheet for one tag row.
type ProjectTagVocabularyActionsMsg struct{ Tag string }

func projectTagVocabularyDescriptors() []OperationDescriptor {
	descriptor := func(id OperationID, label, description string) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Category:             OperationCategoryProject,
			TargetSource:         OperationTargetSelectedEntities,
			TargetCardinality:    TargetCardinalityOneOrMore,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        projectBulkTagsApplicable,
			Risk:                 OperationRiskMutation,
		}
	}
	return []OperationDescriptor{
		withValueProvider(
			descriptor(operationProjectTagRenameAll, "Rename everywhere", "Rename this tag on every project carrying it"),
			ValueProviderTagName,
		),
		withValueProvider(
			descriptor(operationProjectTagRemoveAll, "Remove everywhere", "Remove this tag from every project carrying it"),
			ValueProviderProjectTagDestination,
		),
		descriptor(operationProjectTagParkAll, "Park all members", "Park every project carrying this tag"),
		descriptor(operationProjectTagUnparkAll, "Unpark all members", "Unpark every project carrying this tag"),
	}
}

func (m *AppModel) registerProjectTagVocabularyOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		ValueProviderProjectTagDestination,
		m.projectTagDestinationValue,
	); err != nil {
		return err
	}
	for _, descriptor := range projectTagVocabularyDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			BulkHandler:          m.executeProjectTagVocabulary,
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

// executeProjectTagVocabulary turns one sheet activation into the existing bulk
// tag request. The subject tag comes from the shell rather than the frozen sheet
// value because a nested value step overwrites that value on its way back.
func (m *AppModel) executeProjectTagVocabulary(
	invocation OperationInvocation,
	targets []OperationTarget,
) OperationHandlerResult {
	if m.projectTagActionSubject == "" {
		return OperationHandlerResult{Err: fmt.Errorf("no project tag is selected")}
	}
	message := ProjectBulkTagsMsg{
		ProjectIDs: make([]string, 0, len(targets)),
		Action:     projectTagVocabularyActions[invocation.Descriptor.ID],
		Tag:        m.projectTagActionSubject,
	}
	for _, target := range targets {
		if target.Entity.Kind != EntityKindProject || target.ProjectID == "" {
			return OperationHandlerResult{Err: fmt.Errorf(
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
			return OperationHandlerResult{Err: fmt.Errorf(
				"%s needs a tag name",
				invocation.Descriptor.Label,
			)}
		}
		message.Replacement = replacement
	}
	m.projectTagActionSubject = ""
	return OperationHandlerResult{Message: message}
}

// openProjectTagVocabularyActions freezes the tag's member projects into the
// sheet context, so the eventual edit names the same projects the sheet counted
// even though the picker beneath it has already closed.
func (m *AppModel) openProjectTagVocabularyActions(tag string) tea.Cmd {
	members := m.projectTagMembers(tag)
	if len(members) == 0 {
		return status.ErrorCmd("no active project carries #%s", tag)
	}
	ctx := m.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	ctx.Selection.EntityIdentities = make([]EntityIdentity, 0, len(members))
	for _, member := range members {
		ctx.Selection.EntityIdentities = append(
			ctx.Selection.EntityIdentities,
			ProjectEntity(member.ID),
		)
	}

	sheet, err := NewContextActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		ctx,
		projectTagVocabularyOperations,
		ContextActionSheetConfig{
			Host:         ContextActionHostOutliner,
			Completion:   ContextActionCompleteCloseSheet,
			Title:        "Tag Actions",
			Subtitle:     "#" + tag,
			ItemIDPrefix: "project-tag-action:",
			Categories:   projectTagActionCategories,
			CategoryFor: func(OperationDescriptor) ListCategoryID {
				return projectTagActionCategory
			},
			SecondaryFor: operationActionSecondaryText,
			Bulk:         true,
			Targets:      len(members),
			EmptyState:   ListEmptyState{Title: "No tag actions available."},
			NoMatchState: ListEmptyState{Title: "No matching actions.", Detail: "Clear or change the search."},
		},
	)
	if err != nil {
		return status.ErrorCmd("%v", err)
	}
	m.projectTagActionSubject = tag
	m.contextActionSheet = sheet
	return nil
}

// projectTagMembers is the tag's editable membership. Archived projects refuse
// tag edits, so they are excluded here rather than failing the whole request
// once the transaction is already being planned.
func (m *AppModel) projectTagMembers(tag string) []*models.Project {
	members := make([]*models.Project, 0)
	for _, project := range m.availableProjects() {
		if project != nil && slices.Contains(project.Tags, tag) {
			members = append(members, project)
		}
	}
	return members
}

// projectTagDestinationValue skips the destination prompt unless removing the
// tag would leave a member carrying no tags at all. Only a stranded project has
// a question to answer; everything else keeps the tags it already has.
func (m *AppModel) projectTagDestinationValue(OperationInvocation) OperationValueProviderResult {
	if len(m.strandedProjectTagMembers()) == 0 {
		return OperationValueProviderResult{Value: "", HasValue: true}
	}
	return OperationValueProviderResult{}
}

func (m *AppModel) strandedProjectTagMembers() []*models.Project {
	tag := m.projectTagActionSubject
	if tag == "" {
		return nil
	}
	stranded := make([]*models.Project, 0)
	for _, member := range m.projectTagMembers(tag) {
		if len(member.Tags) == 1 {
			stranded = append(stranded, member)
		}
	}
	return stranded
}

// newProjectTagDestinationPicker asks which tag the stranded members inherit.
// Leaving them untagged is a legitimate answer, so it is a row rather than a
// cancellation.
func (m *AppModel) newProjectTagDestinationPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	subject := m.projectTagActionSubject
	if subject == "" {
		return nil, fmt.Errorf("no project tag is selected")
	}
	counts, _ := m.projectTagVocabularyCounts()
	names := make([]string, 0, len(counts))
	for name := range counts {
		if name != subject && counts[name] > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	items := []ListItem{{
		ID:            "tag-destination:",
		CategoryID:    tagDescriptiveCategory,
		PrimaryText:   "Leave untagged",
		SecondaryText: "members keep no tags",
		Aliases:       []string{"none", "untagged"},
		Data:          "",
	}}
	for _, name := range names {
		items = append(items, ListItem{
			ID:            ListItemID("tag-destination:" + name),
			CategoryID:    tagDescriptiveCategory,
			PrimaryText:   name,
			SecondaryText: projectCountPhrase(counts[name]),
			Aliases:       []string{name, "#" + name},
			Data:          name,
		})
	}

	stranded := m.strandedProjectTagMembers()
	subtitle := fmt.Sprintf(
		"%s would lose every tag",
		projectCountPhrase(len(stranded)),
	)
	return newOperationValuePickerModel(
		request,
		ValueProviderProjectTagDestination,
		"Remove #"+subject+" everywhere",
		subtitle,
		ListSource{
			Items:      items,
			Categories: []ListCategory{{ID: tagDescriptiveCategory, Label: "Destination"}},
		},
		false,
		nil,
	)
}

// projectTagVocabularySelectableTag reports the tag a picker row names, or false
// for the Untagged pseudo-row, which is a facet rather than a tag the catalog
// could rename or remove.
func projectTagVocabularySelectableTag(item ListItem) (string, bool) {
	tag, ok := item.Data.(string)
	if !ok || tag == "" || tag == query.UntaggedProjectFilterKey {
		return "", false
	}
	return strings.ToLower(tag), true
}
