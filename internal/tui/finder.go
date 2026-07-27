package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const (
	// FinderMaxRecentDestinations is the maximum number of live destinations
	// shown before contextual operations in an empty finder.
	FinderMaxRecentDestinations = 3

	// FinderMaxPinnedDestinations is the maximum number of pinned live
	// destinations shown after recents in an empty finder.
	FinderMaxPinnedDestinations = 3

	// FinderEmptyResultLimit is the hard cap for the empty-query shortlist.
	FinderEmptyResultLimit = 8
)

// FinderResult is one final ranked or contextual finder result.
type FinderResult struct {
	Item       ListItem
	Score      int
	Recent     bool
	Pinned     bool
	Contextual bool
}

// Target returns the typed finder payload when the item came from one of the
// built-in providers.
func (result FinderResult) Target() (FinderTarget, bool) {
	target, ok := result.Item.Data.(FinderTarget)
	return target, ok
}

// FinderResultSet is the complete headless response for one query.
type FinderResultSet struct {
	Query     ListQuery
	Results   []FinderResult
	Shortlist bool
}

// Finder owns an immutable unified catalog and reads/writes recency directly
// from one machine-local state. It has no Bubble Tea or renderer dependency.
type Finder struct {
	machine *config.MachineState
	items   []ListItem
	byID    map[ListItemID]ListItem
}

// NewFinder builds and validates one unified index from typed providers.
func NewFinder(
	machine *config.MachineState,
	providers ...FinderProvider,
) (*Finder, error) {
	items := make([]ListItem, 0)
	for index, provider := range providers {
		if provider == nil {
			continue
		}
		provided, err := provider.FinderItems()
		if err != nil {
			return nil, fmt.Errorf("finder provider %d: %w", index, err)
		}
		items = append(items, provided...)
	}

	// Reuse List Surface validation so finder IDs and categories obey the same
	// stable identity contract as their future renderer.
	surface, err := NewListSurfaceModel(items, FinderCategories(), ListSurfaceConfig{})
	if err != nil {
		return nil, fmt.Errorf("build finder index: %w", err)
	}
	normalized := surface.Items()
	byID := make(map[ListItemID]ListItem, len(normalized))
	for _, item := range normalized {
		byID[item.ID] = item
	}
	if machine == nil {
		machine = config.NewMachineState()
	}
	return &Finder{
		machine: machine,
		items:   normalized,
		byID:    byID,
	}, nil
}

// Items returns the complete source catalog in provider order.
func (finder *Finder) Items() []ListItem {
	if finder == nil {
		return []ListItem{}
	}
	return cloneListItems(finder.items)
}

// Results returns the final result set. Empty input returns the bounded
// contextual shortlist; any typed input searches the complete catalog.
func (finder *Finder) Results(query string) FinderResultSet {
	parsed := NewListQuery(query)
	if finder == nil {
		return FinderResultSet{
			Query:     parsed,
			Results:   []FinderResult{},
			Shortlist: parsed.Normalized == "",
		}
	}
	if parsed.Normalized == "" {
		return FinderResultSet{
			Query:     parsed,
			Results:   finder.contextualShortlist(),
			Shortlist: true,
		}
	}

	results := make([]FinderResult, 0, len(finder.items))
	for _, item := range finder.items {
		score := finderItemScore(item, parsed.Normalized)
		if score == 0 {
			continue
		}
		results = append(results, FinderResult{
			Item:   cloneListItem(item),
			Score:  score,
			Pinned: finderItemPinned(item),
		})
	}
	sortFinderResults(results)
	return FinderResultSet{
		Query:   parsed,
		Results: results,
	}
}

// Search is an alias for Results for callers that model finder lookup as a
// search operation.
func (finder *Finder) Search(query string) FinderResultSet {
	return finder.Results(query)
}

// ContextualShortlist returns the final empty-query result set: at most three
// recent live destinations followed by applicable operations, capped at eight.
func (finder *Finder) ContextualShortlist() []FinderResult {
	if finder == nil {
		return []FinderResult{}
	}
	return cloneFinderResults(finder.contextualShortlist())
}

// RecordDestination moves a built-in non-operation target to the front of the
// MachineState-backed qualified history. The owning runtime remains
// responsible for committing that machine domain after a successful action.
func (finder *Finder) RecordDestination(id ListItemID) bool {
	if finder == nil || finder.machine == nil {
		return false
	}
	item, ok := finder.byID[id]
	if !ok {
		return false
	}
	target, ok := item.Data.(FinderTarget)
	if !ok {
		return false
	}
	destination, ok := target.RecentDestination()
	if !ok || destination.QualifiedID() != string(id) {
		return false
	}
	return finder.machine.RecordRecentDestination(destination)
}

func (finder *Finder) contextualShortlist() []FinderResult {
	results := make([]FinderResult, 0, FinderEmptyResultLimit)
	seen := make(map[ListItemID]bool, FinderEmptyResultLimit)

	for _, destination := range finder.machine.GetRecentDestinations() {
		if len(results) == FinderMaxRecentDestinations {
			break
		}
		id := ListItemID(destination.QualifiedID())
		item, ok := finder.byID[id]
		if !ok || seen[id] || !item.Enabled() || item.CategoryID == FinderCategoryOperations {
			continue
		}
		target, targetOK := item.Data.(FinderTarget)
		if !targetOK {
			continue
		}
		liveDestination, destinationOK := target.RecentDestination()
		if !destinationOK || liveDestination.QualifiedID() != destination.QualifiedID() {
			continue
		}
		seen[id] = true
		results = append(results, FinderResult{
			Item:   cloneListItem(item),
			Recent: true,
			Pinned: finderItemPinned(item),
		})
	}

	pinnedCount := 0
	for _, item := range finder.items {
		if len(results) == FinderEmptyResultLimit ||
			pinnedCount == FinderMaxPinnedDestinations {
			break
		}
		if seen[item.ID] || !item.Enabled() ||
			item.CategoryID == FinderCategoryOperations ||
			!finderItemPinned(item) {
			continue
		}
		seen[item.ID] = true
		pinnedCount++
		results = append(results, FinderResult{
			Item:   cloneListItem(item),
			Pinned: true,
		})
	}

	for _, item := range finder.items {
		if len(results) == FinderEmptyResultLimit {
			break
		}
		if item.CategoryID != FinderCategoryOperations || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		results = append(results, FinderResult{
			Item:       cloneListItem(item),
			Contextual: true,
		})
	}
	return results
}

func finderItemScore(item ListItem, query string) int {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0
	}
	primary := strings.ToLower(strings.TrimSpace(item.PrimaryText))
	if primary == query {
		// Every exact primary-label match occupies one fixed tier. Fuzzy
		// implementation details and repeated aliases must not disturb the
		// deterministic category/label/ID tie-break below.
		return 2_000_000
	}
	searchable := make([]string, 0, len(item.Aliases)+2)
	searchable = append(searchable, item.PrimaryText, item.SecondaryText)
	searchable = append(searchable, item.Aliases...)
	score := fuzzy.ScoreTerms(strings.Join(searchable, " "), query)
	if score == 0 {
		return 0
	}

	switch {
	case strings.HasPrefix(primary, query):
		score += 100_000
	case strings.Contains(primary, query):
		score += 10_000
	}
	for _, alias := range item.Aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		switch {
		case alias == query:
			score += 50_000
		case strings.HasPrefix(alias, query):
			score += 5_000
		}
	}
	return score
}

func sortFinderResults(results []FinderResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Pinned != results[j].Pinned {
			return results[i].Pinned
		}
		leftCategory := finderCategoryRank(results[i].Item.CategoryID)
		rightCategory := finderCategoryRank(results[j].Item.CategoryID)
		if leftCategory != rightCategory {
			return leftCategory < rightCategory
		}
		left := strings.ToLower(results[i].Item.PrimaryText)
		right := strings.ToLower(results[j].Item.PrimaryText)
		if left != right {
			return left < right
		}
		return results[i].Item.ID < results[j].Item.ID
	})
}

func finderCategoryRank(id ListCategoryID) int {
	for index, category := range finderCategoryCatalog {
		if category.ID == id {
			return index
		}
	}
	return len(finderCategoryCatalog)
}

func finderItemPinned(item ListItem) bool {
	target, ok := item.Data.(FinderTarget)
	if !ok {
		return false
	}
	switch target.Kind {
	case FinderItemTask:
		return target.Task != nil && target.Task.Pinned
	case FinderItemProject:
		return target.Project != nil && target.Project.Pinned
	case FinderItemNote:
		return target.Note != nil && target.Note.Pinned
	case FinderItemTag:
		return target.Tag != nil && target.Tag.Pinned
	default:
		return false
	}
}

func cloneFinderResults(results []FinderResult) []FinderResult {
	cloned := make([]FinderResult, 0, len(results))
	for _, result := range results {
		result.Item = cloneListItem(result.Item)
		cloned = append(cloned, result)
	}
	return cloned
}

// NewRuntimeFinder assembles every built-in provider from a runtime snapshot.
// Project load failures are skipped, matching the existing discovery surfaces:
// one unavailable project must not prevent the global finder from opening.
func NewRuntimeFinder(
	runtime *config.RuntimeState,
	registry *OperationRegistry,
	context CommandContext,
) (*Finder, error) {
	if runtime == nil {
		return nil, fmt.Errorf("build runtime finder: runtime is required")
	}

	activeProjects := runtime.ActiveProjects()
	loadedProjects := make([]*models.Project, 0, len(activeProjects))
	notes := make([]FinderNote, 0)
	for _, project := range activeProjects {
		if project == nil || storage.LoadProject(project) != nil {
			continue
		}
		loadedProjects = append(loadedProjects, project)
		projectNotes, err := storage.ListNotes(project)
		if err != nil {
			continue
		}
		for _, note := range projectNotes {
			notes = append(notes, FinderNote{Note: note, Project: project})
		}
	}
	globalNotes, err := storage.ListNotesFrom(storage.GlobalNotesDir(runtime.DataDir()))
	if err == nil {
		for _, note := range globalNotes {
			notes = append(notes, FinderNote{Note: note})
		}
	}

	tools := finderTools()
	tags := finderTagsFromProjects(runtime, loadedProjects)
	filters := runtime.Planning().GetSavedFilters()
	providers := []FinderProvider{
		FinderOperationProvider{Registry: registry, Context: context},
		FinderTaskProvider{Projects: loadedProjects},
		FinderProjectProvider{Projects: projectDiscoveryProjects(runtime)},
		FinderNoteProvider{Notes: notes},
		FinderToolProvider{Tools: tools},
		FinderModeProvider{Modes: OutlinerModeCatalog()},
		FinderScopeProvider{Scopes: ContextScopeSummaries(runtime)},
		FinderTagProvider{Tags: tags},
		FinderSavedFilterProvider{Filters: filters},
	}
	return NewFinder(runtime.Machine(), providers...)
}

func finderTools() []FinderTool {
	return []FinderTool{
		{ID: "weekly-review", Label: "Weekly Review", Description: "Strategic project health review"},
		{ID: "metrics", Label: "Metrics", Description: "Completion, cycle-time, and throughput analytics"},
		{ID: "roadmap", Label: "Roadmap", Description: "Timelines, forecasts, and critical paths"},
		{ID: "blockers", Label: "Blockers", Description: "Blocked and blocking tasks"},
		{ID: "notes", Label: "Notes", Description: "Project and global notes"},
		{ID: "tags", Label: "Tags", Description: "Manage tags across projects"},
		{ID: "workflow-rules", Label: "Workflow Rules", Description: "Manage workflow automation rules"},
		{ID: "settings", Label: "Settings", Description: "Application preferences and configuration"},
		{ID: "contexts", Label: "Contexts", Description: "Browse and apply project contexts"},
	}
}

func finderTagsFromProjects(runtime *config.RuntimeState, projects []*models.Project) []TagInfo {
	counts := make(map[string]int)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for tag, count := range project.TagCounts() {
			counts[tag] += count
		}
	}
	tags := make([]TagInfo, 0, len(counts))
	for tag, count := range counts {
		tags = append(tags, TagInfo{
			Name:      tag,
			TaskCount: count,
			Pinned:    runtime.Settings().IsGlobalTagPinned(tag),
		})
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	return tags
}
