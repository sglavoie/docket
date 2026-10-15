package projecthub

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/theme"
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
// first, then the parked projects it leaves out, then the archived tail. The
// pinned Hub is one group, since a pin is what it lists and a parked pin keeps
// its `#parked` chip there instead.
func (m *ProjectHubModel) projectHubCategories() []listsurface.ListCategory {
	if m.mode == route.ProjectHubModePinned {
		return []listsurface.ListCategory{{ID: projectHubCategoryActive, Label: "Pinned Projects"}}
	}
	return []listsurface.ListCategory{
		{ID: projectHubCategoryActive, Label: "Active Projects"},
		{ID: projectHubCategoryParked, Label: "Parked Projects"},
		{ID: projectHubCategoryArchived, Label: "Archived Projects"},
	}
}

// projectHubGroupsParked reports whether a project draws under the Parked
// group. Only the full Hub has one; the pinned Hub lists every pin together.
func (m *ProjectHubModel) projectHubGroupsParked(project *models.Project) bool {
	return m.mode != route.ProjectHubModePinned && project != nil && models.ProjectTagsParked(project.Tags)
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
	now := time.Now()
	for _, project := range m.runtime.AvailableProjects() {
		if project == nil || (m.mode == route.ProjectHubModePinned && !project.Pinned) || m.projectHubFilterHides(project) {
			continue
		}
		label := project.Name
		if project.ID == m.focusedProjectID {
			label += " (focused)"
		}
		category, lifecycle, phase := projectHubCategoryActive, "active project", projectPreviewActive
		var primaryStyle *lipgloss.Style
		if m.projectHubGroupsParked(project) {
			// A parked row is dimmed rather than restyled: it is still a live
			// project every act reaches, only one the base Outliner leaves out.
			category, lifecycle, phase = projectHubCategoryParked, "parked project", projectPreviewParked
			muted := theme.MutedStyle
			primaryStyle = &muted
		}
		phrase := m.projectTaskPhraseAt(project, now)
		items = append(items, listsurface.ListItem{
			ID:                   listsurface.ListItemID(projectHubItemPrefix + project.ID),
			CategoryID:           category,
			PrimaryText:          label,
			PrimaryStyle:         primaryStyle,
			PrimaryMarker:        projectPinnedMarker(project),
			SecondaryText:        joinProjectSecondary(phrase.text, ProjectTagChips(m.projectRowTags(project))),
			SecondaryAccent:      phrase.overdue,
			SecondaryAccentStyle: projectOverdueStyle(phrase.overdue),
			Aliases:              projectHubAliases(project, lifecycle),
			Preview:              m.projectPreview(project, phase, phrase.text),
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
			Preview:       m.projectPreview(project, projectPreviewArchived, m.projectTaskPhraseAt(project, now).text),
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

// projectPreviewPhase is the lifecycle word the preview header leads with.
type projectPreviewPhase string

const (
	projectPreviewActive   projectPreviewPhase = "Active"
	projectPreviewParked   projectPreviewPhase = "Parked"
	projectPreviewArchived projectPreviewPhase = "Archived"
)

// projectPreview spends two rows on the project's identity and leaves the rest
// of the pane to its tasks. The five labeled fields this replaced cost six rows
// to say what one muted line says, and `Pinned: false` said nothing at all: an
// unpinned project is the ordinary case, so the marker appears only when it is
// true. The line travels as segments, so a narrow pane wraps between them and
// `slug agent-mux` never splits. taskPhrase is the row's own tally wording, so
// the two never disagree.
func (m *ProjectHubModel) projectPreview(project *models.Project, phase projectPreviewPhase, taskPhrase string) listsurface.ListPreview {
	meta := []string{string(phase), taskPhrase, "slug " + project.Slug}
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

// joinProjectSecondary carries the task tally and the project's own tag chips.
// Both numbers ride the row because a total alone reads as pending work when
// every task in it is finished, and the open count leads because a narrow row
// truncates secondary text from the right. A known-empty project has no open
// subset worth naming and keeps its bare total. The chips are plain text rather
// than styled badges because the row layout measures this string.
func joinProjectSecondary(phrase, chips string) string {
	if chips == "" {
		return phrase
	}
	return phrase + " · " + chips
}

// projectRowTags is the tag set a row's chips spell. A row under the Parked
// group drops `parked`, since the group heading already says it; every other
// row, the pinned Hub's included, keeps the chip as the one place it is said.
func (m *ProjectHubModel) projectRowTags(project *models.Project) []string {
	if !m.projectHubGroupsParked(project) {
		return project.Tags
	}
	tags := make([]string, 0, len(project.Tags))
	for _, tag := range project.Tags {
		if tag != models.ParkedProjectTag {
			tags = append(tags, tag)
		}
	}
	return tags
}

// projectTaskPhrase is a project's tally in words, and overdue names the run
// inside it that counts missed deadlines, empty when there are none, so the row
// can paint that run as a warning without reparsing the text.
type projectTaskPhrase struct {
	text    string
	overdue string
}

// projectTaskPhraseAt is the one wording of a project's tally, so the row and
// the preview header never disagree about how much work a project holds. The
// overdue count sits beside the open count it is a part of, ahead of the total,
// so a narrow row sheds the total before the warning.
func (m *ProjectHubModel) projectTaskPhraseAt(project *models.Project, now time.Time) projectTaskPhrase {
	tally, ok := m.projectTally(project)
	switch {
	case !ok:
		return projectTaskPhrase{text: "tasks not loaded"}
	case tally.Total == 0:
		return projectTaskPhrase{text: "0 tasks"}
	}
	phrase := projectTaskPhrase{text: fmt.Sprintf("%d open", tally.Open)}
	if overdue := tally.OverdueAt(now); overdue > 0 {
		phrase.overdue = fmt.Sprintf("%d overdue", overdue)
		phrase.text += " · " + phrase.overdue
	}
	phrase.text += " · " + util.Count(tally.Total, "task")
	return phrase
}

// projectOverdueStyle is the paint for the overdue run: the Outliner project
// header's own overdue pressure color, so one fact keeps one color.
func projectOverdueStyle(overdue string) *lipgloss.Style {
	if overdue == "" {
		return nil
	}
	style := theme.ProjectPressureOverdueStyle
	return &style
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
