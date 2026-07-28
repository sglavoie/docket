package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// FinderItemKind identifies the typed payload carried by a finder row.
type FinderItemKind string

const (
	FinderItemOperation   FinderItemKind = "operation"
	FinderItemTask        FinderItemKind = "task"
	FinderItemProject     FinderItemKind = "project"
	FinderItemNote        FinderItemKind = "note"
	FinderItemTool        FinderItemKind = "tool"
	FinderItemMode        FinderItemKind = "mode"
	FinderItemScope       FinderItemKind = "scope"
	FinderItemTag         FinderItemKind = "tag"
	FinderItemSavedFilter FinderItemKind = "saved-filter"
)

const (
	FinderCategoryOperations   ListCategoryID = "finder:operations"
	FinderCategoryTasks        ListCategoryID = "finder:tasks"
	FinderCategoryProjects     ListCategoryID = "finder:projects"
	FinderCategoryNotes        ListCategoryID = "finder:notes"
	FinderCategoryTools        ListCategoryID = "finder:tools"
	FinderCategoryModes        ListCategoryID = "finder:modes"
	FinderCategoryScopes       ListCategoryID = "finder:scopes"
	FinderCategoryTags         ListCategoryID = "finder:tags"
	FinderCategorySavedFilters ListCategoryID = "finder:saved-filters"
)

var finderCategoryCatalog = []ListCategory{
	{ID: FinderCategoryOperations, Label: "Operations"},
	{ID: FinderCategoryTasks, Label: "Tasks"},
	{ID: FinderCategoryProjects, Label: "Projects"},
	{ID: FinderCategoryNotes, Label: "Notes"},
	{ID: FinderCategoryTools, Label: "Tools"},
	{ID: FinderCategoryModes, Label: "Modes"},
	{ID: FinderCategoryScopes, Label: "Scopes"},
	{ID: FinderCategoryTags, Label: "Tags"},
	{ID: FinderCategorySavedFilters, Label: "Saved Filters"},
}

// FinderCategories returns the fixed category order shared by every finder.
func FinderCategories() []ListCategory {
	return append([]ListCategory(nil), finderCategoryCatalog...)
}

// FinderTarget is the typed destination or operation carried by ListItem.Data.
// Recent is empty for operations and is fully project-qualified for task and
// project-note destinations.
type FinderTarget struct {
	Kind        FinderItemKind
	Recent      config.RecentDestination
	Operation   *OperationDescriptor
	Task        *models.Task
	Project     *models.Project
	Note        *models.Note
	Tool        *FinderTool
	Mode        *OutlinerMode
	Scope       *ContextScopeSummary
	Tag         *TagInfo
	SavedFilter *config.SavedFilter
}

// RecentDestination returns the durable destination identity when this target
// may appear in the finder recent shortlist.
func (target FinderTarget) RecentDestination() (config.RecentDestination, bool) {
	if target.Kind == FinderItemOperation || !target.Recent.Valid() {
		return config.RecentDestination{}, false
	}
	return target.Recent, true
}

// FinderProvider contributes one typed catalog to a headless Finder.
type FinderProvider interface {
	FinderItems() ([]ListItem, error)
}

// FinderOperationProvider exposes only operations applicable to its typed
// command context.
type FinderOperationProvider struct {
	Registry *OperationRegistry
	Context  CommandContext
}

// FinderItems implements FinderProvider.
func (provider FinderOperationProvider) FinderItems() ([]ListItem, error) {
	if provider.Registry == nil {
		return []ListItem{}, nil
	}
	descriptors := finderOperationDescriptors(provider.Registry, provider.Context)
	items := make([]ListItem, 0, len(descriptors))
	for _, descriptor := range descriptors {
		descriptorCopy := descriptor
		secondary := string(descriptor.Category)
		if descriptor.DirectKey != "" {
			secondary += " · " + descriptor.DirectKey
		}
		items = append(items, ListItem{
			ID:            ListItemID("operation:" + string(descriptor.ID)),
			CategoryID:    FinderCategoryOperations,
			PrimaryText:   descriptor.Label,
			SecondaryText: secondary,
			Aliases:       append([]string(nil), descriptor.Aliases...),
			OperationIDs:  []OperationID{descriptor.ID},
			Preview: ListPreview{
				Title: descriptor.Label,
				Fields: []ListPreviewField{
					{Label: "Category", Value: string(descriptor.Category)},
					{Label: "Risk", Value: string(descriptor.Risk)},
					{Label: "Target", Value: string(descriptor.TargetCardinality)},
				},
			},
			Data: FinderTarget{
				Kind:      FinderItemOperation,
				Operation: &descriptorCopy,
			},
		})
	}
	return items, nil
}

// finderOperationDescriptors decides membership from descriptor placement and
// applicability, so every production operation the context admits is listed
// exactly once and tool descriptors stay in the Tools category.
//
// The active key grammar only decides order: operations the Outliner also binds
// to a direct key lead, which is what keeps the bounded empty-query shortlist on
// the daily loop instead of the alphabetical head of the catalog.
func finderOperationDescriptors(
	registry *OperationRegistry,
	ctx CommandContext,
) []OperationDescriptor {
	discovered := make([]OperationDescriptor, 0)
	for _, descriptor := range registry.All() {
		if descriptor.Placement == PlacementFinderOperation && descriptor.Applicable(ctx) {
			discovered = append(discovered, descriptor)
		}
	}
	ranks := make(map[OperationID]int)
	unbound := 0
	if strategy, ok := registry.InputStrategy(InputStrategySimplified); ok {
		for index, binding := range strategy.Bindings {
			if _, exists := ranks[binding.OperationID]; !exists {
				ranks[binding.OperationID] = index
			}
		}
		unbound = len(strategy.Bindings)
	}
	rankOf := func(id OperationID) int {
		if index, ok := ranks[id]; ok {
			return index
		}
		return unbound
	}
	sort.SliceStable(discovered, func(i, j int) bool {
		return rankOf(discovered[i].ID) < rankOf(discovered[j].ID)
	})
	return discovered
}

// FinderTaskProvider exposes active, nonterminal tasks. Its IDs use the
// project-qualified identity established by List Surface.
type FinderTaskProvider struct {
	Projects []*models.Project
}

// FinderItems implements FinderProvider.
func (provider FinderTaskProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0)
	for _, project := range provider.Projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.IsTerminalState() {
				continue
			}
			state := task.State()
			aliases := []string{
				task.Title,
				fmt.Sprintf("#%d", task.ShortID),
				state.String(),
				project.Name,
				project.Slug,
			}
			aliases = append(aliases, task.Tags...)
			items = append(items, ListItem{
				ID:            QualifiedTaskListItemID(project.ID, task.ID),
				CategoryID:    FinderCategoryTasks,
				PrimaryText:   task.Title,
				SecondaryText: fmt.Sprintf("#%d · %s · %s", task.ShortID, project.Name, workflowStateListLabel(state)),
				Aliases:       aliases,
				OperationIDs:  taskOperationManifestIDs(),
				Preview: ListPreview{
					Title:   task.Title,
					Summary: task.Description,
					Fields: []ListPreviewField{
						{Label: "Project", Value: project.Name},
						{Label: "State", Value: state.String()},
						{Label: "Priority", Value: strconv.Itoa(task.Priority)},
						{Label: "Tags", Value: strings.Join(task.Tags, ", ")},
					},
				},
				Data: FinderTarget{
					Kind:    FinderItemTask,
					Task:    task,
					Project: project,
					Recent: config.RecentDestination{
						Kind:      config.RecentDestinationTask,
						ProjectID: project.ID,
						ID:        task.ID,
					},
				},
			})
		}
	}
	sortFinderProviderItems(items)
	return items, nil
}

// FinderProjectProvider exposes active and archived project destinations.
type FinderProjectProvider struct {
	Projects []*models.Project
}

// FinderItems implements FinderProvider.
func (provider FinderProjectProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0, len(provider.Projects))
	for _, project := range provider.Projects {
		if project == nil {
			continue
		}
		status := "Active"
		if project.Archived {
			status = "Archived"
		} else if project.Pinned {
			status = "Pinned"
		}
		items = append(items, ListItem{
			ID:            ListItemID("project:" + project.ID),
			CategoryID:    FinderCategoryProjects,
			PrimaryText:   project.Name,
			SecondaryText: project.Context + " · " + status,
			Aliases:       []string{project.Name, project.Slug, project.Context, status},
			OperationIDs:  operationDescriptorIDs(projectOperationDescriptors()),
			Preview: ListPreview{
				Title:   project.Name,
				Summary: project.Description,
				Fields: []ListPreviewField{
					{Label: "Slug", Value: project.Slug},
					{Label: "Context", Value: project.Context},
					{Label: "Status", Value: status},
				},
			},
			Data: FinderTarget{
				Kind:    FinderItemProject,
				Project: project,
				Recent: config.RecentDestination{
					Kind: config.RecentDestinationProject,
					ID:   project.ID,
				},
			},
		})
	}
	sortFinderProviderItems(items)
	return items, nil
}

// FinderNote pairs a note with its optional owning project. A nil project
// identifies a workspace-global note.
type FinderNote struct {
	Note    *models.Note
	Project *models.Project
}

// FinderNoteProvider exposes project and global notes.
type FinderNoteProvider struct {
	Notes []FinderNote
}

// FinderItems implements FinderProvider.
func (provider FinderNoteProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0, len(provider.Notes))
	for _, entry := range provider.Notes {
		if entry.Note == nil {
			continue
		}
		projectID := ""
		scope := "Global"
		aliases := append([]string{entry.Note.Title, entry.Note.Filename}, entry.Note.Tags...)
		if entry.Project != nil {
			projectID = entry.Project.ID
			scope = entry.Project.Name
			aliases = append(aliases, entry.Project.Name, entry.Project.Slug)
		}
		items = append(items, ListItem{
			ID:            QualifiedNoteListItemID(projectID, entry.Note.Filename),
			CategoryID:    FinderCategoryNotes,
			PrimaryText:   entry.Note.Title,
			SecondaryText: scope,
			Aliases:       aliases,
			OperationIDs:  operationDescriptorIDs(noteOperationDescriptors()),
			Preview: ListPreview{
				Title:   entry.Note.Title,
				Summary: entry.Note.Filename,
				Body:    entry.Note.Body,
				Fields: []ListPreviewField{
					{Label: "Scope", Value: scope},
					{Label: "Tags", Value: strings.Join(entry.Note.Tags, ", ")},
				},
			},
			Data: FinderTarget{
				Kind:    FinderItemNote,
				Note:    entry.Note,
				Project: entry.Project,
				Recent: config.RecentDestination{
					Kind:      config.RecentDestinationNote,
					ProjectID: projectID,
					ID:        entry.Note.Filename,
				},
			},
		})
	}
	sortFinderProviderItems(items)
	return items, nil
}

// FinderTool is a directly openable secondary tool or consolidated surface.
type FinderTool struct {
	ID          string
	Label       string
	Description string
	Aliases     []string
}

// FinderToolProvider exposes directly openable secondary tools.
type FinderToolProvider struct {
	Tools []FinderTool
}

// FinderItems implements FinderProvider.
func (provider FinderToolProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0, len(provider.Tools))
	for _, tool := range provider.Tools {
		id := strings.TrimSpace(tool.ID)
		if id == "" {
			continue
		}
		toolCopy := tool
		items = append(items, ListItem{
			ID:            ListItemID("tool:" + id),
			CategoryID:    FinderCategoryTools,
			PrimaryText:   tool.Label,
			SecondaryText: tool.Description,
			Aliases:       append([]string(nil), tool.Aliases...),
			OperationIDs:  []OperationID{OperationID(id)},
			Preview: ListPreview{
				Title:   tool.Label,
				Summary: tool.Description,
			},
			Data: FinderTarget{
				Kind: FinderItemTool,
				Tool: &toolCopy,
				Recent: config.RecentDestination{
					Kind: config.RecentDestinationTool,
					ID:   id,
				},
			},
		})
	}
	sortFinderProviderItems(items)
	return items, nil
}

// FinderModeProvider exposes the Outliner mode catalog.
type FinderModeProvider struct {
	Modes []OutlinerMode
}

// FinderItems implements FinderProvider.
func (provider FinderModeProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0, len(provider.Modes))
	for _, mode := range provider.Modes {
		modeCopy := mode
		ordering := "Manual order"
		if mode.FixedOrder() {
			ordering = "Fixed order"
		}
		items = append(items, ListItem{
			ID:             ListItemID("mode:" + string(mode.ID)),
			CategoryID:     FinderCategoryModes,
			PrimaryText:    mode.Label,
			SecondaryText:  mode.Category.String() + " · " + ordering,
			Aliases:        append([]string{string(mode.ID)}, mode.Aliases...),
			OperationIDs:   []OperationID{operationModeActivate},
			DisabledReason: finderModeDisabledReason(mode),
			Preview: ListPreview{
				Title: mode.Label,
				Fields: []ListPreviewField{
					{Label: "Category", Value: mode.Category.String()},
					{Label: "Ordering", Value: ordering},
				},
			},
			Data: FinderTarget{
				Kind: FinderItemMode,
				Mode: &modeCopy,
				Recent: config.RecentDestination{
					Kind: config.RecentDestinationMode,
					ID:   string(mode.ID),
				},
			},
		})
	}
	sortFinderProviderItems(items)
	return items, nil
}

// FinderScopeProvider exposes every canonical project scope.
type FinderScopeProvider struct {
	Scopes []ContextScopeSummary
}

// FinderItems implements FinderProvider.
func (provider FinderScopeProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0, len(provider.Scopes))
	for _, summary := range provider.Scopes {
		summaryCopy := summary
		label := summary.Scope.Name
		if summary.Scope.IsAll() {
			label = "All Projects"
		}
		id := strings.TrimPrefix(string(contextScopeListItemID(summary.Scope)), "scope:")
		items = append(items, ListItem{
			ID:            ListItemID("scope:" + id),
			CategoryID:    FinderCategoryScopes,
			PrimaryText:   label,
			SecondaryText: fmt.Sprintf("%d projects", summary.ActiveProjectCount),
			Aliases:       []string{summary.Scope.Name, "scope", "projects"},
			OperationIDs:  finderScopeOperationIDs(summary.Scope),
			Preview: ListPreview{
				Title: label,
				Fields: []ListPreviewField{
					{Label: "Active", Value: strconv.Itoa(summary.ActiveProjectCount)},
					{Label: "Archived", Value: strconv.Itoa(summary.ArchivedProjectCount)},
					{Label: "Missing", Value: strconv.Itoa(summary.StaleReferenceCount)},
				},
			},
			Data: FinderTarget{
				Kind:   FinderItemScope,
				Scope:  &summaryCopy,
				Recent: recentDestinationForContextScope(summary.Scope),
			},
		})
	}
	sortFinderProviderItems(items)
	return items, nil
}

// FinderTagProvider exposes a scope's aggregate tag catalog.
type FinderTagProvider struct {
	Tags []TagInfo
}

// FinderItems implements FinderProvider.
func (provider FinderTagProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0, len(provider.Tags))
	for _, tag := range provider.Tags {
		tagCopy := tag
		items = append(items, ListItem{
			ID:            ListItemID("tag:" + tag.Name),
			CategoryID:    FinderCategoryTags,
			PrimaryText:   tag.Name,
			SecondaryText: fmt.Sprintf("%d tasks", tag.TaskCount),
			Aliases:       []string{tag.Name, "#" + tag.Name},
			OperationIDs:  operationDescriptorIDs(tagOperationDescriptors()),
			Preview: ListPreview{
				Title: tag.Name,
				Fields: []ListPreviewField{
					{Label: "Tasks", Value: strconv.Itoa(tag.TaskCount)},
					{Label: "Pinned", Value: strconv.FormatBool(tag.Pinned)},
				},
			},
			Data: FinderTarget{
				Kind: FinderItemTag,
				Tag:  &tagCopy,
				Recent: config.RecentDestination{
					Kind: config.RecentDestinationTag,
					ID:   tag.Name,
				},
			},
		})
	}
	sortFinderProviderItems(items)
	return items, nil
}

// FinderSavedFilterProvider exposes durable saved filters.
type FinderSavedFilterProvider struct {
	Filters []config.SavedFilter
}

// FinderItems implements FinderProvider.
func (provider FinderSavedFilterProvider) FinderItems() ([]ListItem, error) {
	items := make([]ListItem, 0, len(provider.Filters))
	for _, filter := range provider.Filters {
		filterCopy := filter
		items = append(items, ListItem{
			ID:            ListItemID("saved-filter:" + filter.ID),
			CategoryID:    FinderCategorySavedFilters,
			PrimaryText:   filter.Name,
			SecondaryText: describeSavedFilterSummary(&filter),
			Aliases:       []string{filter.Name, filter.Mode, filter.FilterType},
			OperationIDs:  operationDescriptorIDs(savedFilterOperationDescriptors()),
			Preview: ListPreview{
				Title:   filter.Name,
				Summary: describeSavedFilterSummary(&filter),
				Fields: []ListPreviewField{
					{Label: "Mode", Value: filter.Mode},
					{Label: "Type", Value: filter.FilterType},
				},
			},
			Data: FinderTarget{
				Kind:        FinderItemSavedFilter,
				SavedFilter: &filterCopy,
				Recent: config.RecentDestination{
					Kind: config.RecentDestinationSavedFilter,
					ID:   filter.ID,
				},
			},
		})
	}
	sortFinderProviderItems(items)
	return items, nil
}

// operationDescriptorIDs projects an ordered descriptor catalog into the row
// manifest a Finder result carries. The contextual sheet drops whatever the
// frozen target cannot do, so a row may safely advertise its whole catalog.
func operationDescriptorIDs(descriptors []OperationDescriptor) []OperationID {
	ids := make([]OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	return ids
}

// finderScopeOperationIDs limits regular and All rows to activation because
// membership, rename, and delete only exist for custom contexts.
func finderScopeOperationIDs(scope ContextScope) []OperationID {
	ids := []OperationID{operationScopeActivateContext}
	if !scope.IsCustom() {
		return ids
	}
	return append(ids,
		operationScopeEditContext,
		operationScopeRenameContext,
		operationScopeDeleteContext,
	)
}

func recentDestinationForContextScope(scope ContextScope) config.RecentDestination {
	id := strings.TrimPrefix(string(contextScopeListItemID(scope)), "scope:")
	return config.RecentDestination{
		Kind: config.RecentDestinationScope,
		ID:   id,
	}
}

// QualifiedNoteListItemID returns a project-qualified note identity. Global
// notes use the explicit "global" scope rather than an empty component.
func QualifiedNoteListItemID(projectID, filename string) ListItemID {
	if projectID == "" {
		projectID = "global"
	}
	return ListItemID("note:" + projectID + ":" + filename)
}

func finderModeDisabledReason(mode OutlinerMode) string {
	if modePickerModeUnavailable(mode) {
		return "not available in this scope"
	}
	return ""
}

func sortFinderProviderItems(items []ListItem) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(items[i].PrimaryText)
		right := strings.ToLower(items[j].PrimaryText)
		if left != right {
			return left < right
		}
		return items[i].ID < items[j].ID
	})
}
