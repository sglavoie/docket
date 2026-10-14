package projecthub

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/util"
)

func (m *ProjectHubModel) rebuildProjectSurface(prefer listsurface.ListItemID) {
	categories := m.projectHubCategories()
	items := m.projectHubItems()
	if m.surface == nil {
		surface, err := listsurface.NewListSurfaceModel(items, categories, listsurface.ListSurfaceConfig{
			SelectionMode: listsurface.ListSelectionMark,
			EmptyState:    m.projectHubEmptyState(),
			NoMatchState:  m.projectHubNoMatchState(),
		})
		if err != nil {
			m.notice = err.Error()
			return
		}
		m.surface = surface
	} else if err := m.surface.Reload(items, categories); err != nil {
		m.notice = err.Error()
		return
	}
	m.refreshProjectHubEmptyStates()
	m.surface.SetQuery(m.SearchQuery())
	if prefer != "" {
		m.surface.SetCursorToID(prefer)
	}
	// The population was rebuilt, so the tasks behind the previewed row may have
	// moved with it; a cached tree would keep painting the ones that are gone.
	m.preview.invalidate()
}

// projectHubCategories orders the listing groups: the base task workspace
// first, then the archived tail.
func (m *ProjectHubModel) projectHubCategories() []listsurface.ListCategory {
	if m.mode == route.ProjectHubModePinned {
		return []listsurface.ListCategory{{ID: projectHubCategoryActive, Label: "Pinned Projects"}}
	}
	return []listsurface.ListCategory{
		{ID: projectHubCategoryActive, Label: "Active Projects"},
		{ID: projectHubCategoryArchived, Label: "Archived Projects"},
	}
}

// projectPinnedMarker gives an archived project the marker too: the pin is real
// state and survives archiving, so the row says so. The act does not follow it —
// operation.OperationScopePinProject requires an active project — which is why the footer
// hint and the `?` help row disappear on an archived row while the marker stays.
func projectPinnedMarker(project *models.Project) string {
	if project == nil || !project.Pinned {
		return ""
	}
	return listsurface.PinnedMarker()
}

// projectHubAliases is the one alias vocabulary every Hub row is indexed under,
// so the active, archived, and pinned populations answer the same query the same
// way. Beyond the name, the slug, and the lifecycle phrase, it carries the
// project's own description and every normalized project tag twice: bare and
// `#`-prefixed, because the chips the row shows are written `#parked` and a
// query that copies what it reads must match.
//
// It carries nothing a task owns. Select Project chooses a project, and a task
// title, description, tag, context, or comment matching a project row would
// promise the row holds work it does not name; the Outliner's `/` is where task
// content is searched.
func projectHubAliases(project *models.Project, lifecycle string) []string {
	aliases := make([]string, 0, 4+2*len(project.Tags))
	aliases = append(aliases, project.Name, project.Slug, lifecycle)
	if description := strings.Join(strings.Fields(project.Description), " "); description != "" {
		aliases = append(aliases, description)
	}
	for _, tag := range project.Tags {
		if tag = strings.TrimSpace(tag); tag == "" {
			continue
		}
		aliases = append(aliases, tag, "#"+tag)
	}
	return aliases
}

func (m *ProjectHubModel) projectHubItems() []listsurface.ListItem {
	items := make([]listsurface.ListItem, 0, len(m.runtime.AllProjects()))
	for _, project := range m.runtime.AvailableProjects() {
		if project == nil || (m.mode == route.ProjectHubModePinned && !project.Pinned) || m.projectHubFilterHides(project) {
			continue
		}
		label := project.Name
		if project.ID == m.focusedProjectID {
			label += " (focused)"
		}
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID(projectHubItemPrefix + project.ID),
			CategoryID:    projectHubCategoryActive,
			PrimaryText:   label,
			PrimaryMarker: projectPinnedMarker(project),
			SecondaryText: m.projectSecondary(project),
			Aliases:       projectHubAliases(project, "active project"),
			Preview:       m.projectPreview(project, false),
			OperationIDs: append([]operation.OperationID{
				operation.OperationScopeArchiveProject,
			}, projectBulkOperations...),
			Data: projectHubEntry{Kind: projectHubEntryActive, Project: project},
		})
	}
	if m.mode == route.ProjectHubModePinned {
		return items
	}
	for _, project := range m.runtime.ArchivedProjects() {
		if project == nil || m.projectHubFilterHides(project) {
			continue
		}
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID(projectHubArchivedItemPrefix + project.ID),
			CategoryID:    projectHubCategoryArchived,
			PrimaryText:   project.Name,
			PrimaryMarker: projectPinnedMarker(project),
			SecondaryText: strings.TrimSuffix("archived · "+ProjectTagChips(project.Tags), " · "),
			Aliases:       projectHubAliases(project, "archived project"),
			Preview:       m.projectPreview(project, true),
			OperationIDs: []operation.OperationID{
				operation.OperationScopeRestoreProject,
			},
			Data: projectHubEntry{Kind: projectHubEntryArchived, Project: project},
		})
	}
	return items
}

// projectHubFilterHides reports the Ctrl+T filter hiding a project. It hides a
// known count only: a project whose tasks could not be read is absent from the
// counts and is not known to be empty or to hold work, so neither narrowing may
// claim it.
func (m *ProjectHubModel) projectHubFilterHides(project *models.Project) bool {
	tally, ok := m.taskCounts[project.ID]
	if !ok {
		return false
	}
	switch m.taskFilter {
	case projectHubTaskFilterWithOpenTasks:
		return tally.Open == 0
	case projectHubTaskFilterWithoutOpenTasks:
		return tally.Open > 0
	default:
		return false
	}
}

// projectPreview spends two rows on the project's identity and leaves the rest
// of the pane to its tasks. The five labeled fields this replaced cost six rows
// to say what one muted line says, and `Pinned: false` said nothing at all: an
// unpinned project is the ordinary case, so the marker appears only when it is
// true. The line travels as segments, so a narrow pane wraps between them and
// `slug agent-mux` never splits.
func (m *ProjectHubModel) projectPreview(project *models.Project, archived bool) listsurface.ListPreview {
	lifecycle := "Active"
	if archived {
		lifecycle = "Archived"
	}
	meta := []string{lifecycle, m.projectTaskPhrase(project), "slug " + project.Slug}
	if project.Pinned {
		meta = append(meta, "pinned")
	}
	return listsurface.ListPreview{
		Title:           project.Name,
		SummarySegments: meta,
		Body:            project.Description,
	}
}

// projectTally reports the loaded task tally, or unknown. A project outside the
// base task workspace is not loaded, and its rehydrated catalog record counts
// zero tasks; reporting that zero would read as "empty" rather than "not
// counted", so an unloaded project is never given a number.
func (m *ProjectHubModel) projectTally(project *models.Project) (ProjectTaskTally, bool) {
	if project == nil {
		return ProjectTaskTally{}, false
	}
	if tally, ok := m.taskCounts[project.ID]; ok {
		return tally, true
	}
	if !project.IsLoaded() {
		return ProjectTaskTally{}, false
	}
	return ProjectTaskTallyOf(project), true
}

// projectSecondary carries the task tally and the project's own tag chips. Both
// numbers ride the row because a total alone reads as pending work when every
// task in it is finished, and the open count leads because a narrow row
// truncates secondary text from the right. A known-empty project has no open
// subset worth naming and keeps its bare total. The chips are plain text rather
// than styled badges because the row layout measures this string, and they are
// what makes a parked project legible here: the Hub lists parked projects
// alongside every other active one.
func (m *ProjectHubModel) projectSecondary(project *models.Project) string {
	secondary := m.projectTaskPhrase(project)
	if chips := ProjectTagChips(project.Tags); chips != "" {
		secondary += " · " + chips
	}
	return secondary
}

// projectTaskPhrase is the one wording of a project's tally, so the row and the
// preview header never disagree about how much work a project holds.
func (m *ProjectHubModel) projectTaskPhrase(project *models.Project) string {
	tally, ok := m.projectTally(project)
	switch {
	case !ok:
		return "tasks not loaded"
	case tally.Total == 0:
		return "0 tasks"
	default:
		return fmt.Sprintf("%d open · %s", tally.Open, util.Count(tally.Total, "task"))
	}
}

func ProjectTagChips(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	chips := make([]string, 0, len(tags))
	for _, tag := range tags {
		chips = append(chips, "#"+tag)
	}
	return strings.Join(chips, " ")
}
