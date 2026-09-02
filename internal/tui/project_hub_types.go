package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ProjectHubMode identifies the population exposed by the Project Hub. All is
// the zero value so existing route and message callers retain the full Hub.
type ProjectHubMode uint8

const (
	ProjectHubModeAll ProjectHubMode = iota
	ProjectHubModePinned
)

// ShowProjectHubMsg opens the searchable project selection and administration
// surface in the requested population mode.
type ShowProjectHubMsg struct {
	Mode ProjectHubMode
}

// The operation ID predates the dedicated Project Hub and remains stable for
// operation-catalog compatibility.
const operationScopeCreateProject OperationID = "tui.scope.create-project"

const (
	projectHubCategoryActive   ListCategoryID = "project-hub:active"
	projectHubCategoryArchived ListCategoryID = "project-hub:archived"

	projectHubActionCategoryProject  ListCategoryID = "project-hub-actions:project"
	projectHubActionCategoryRecovery ListCategoryID = "project-hub-actions:recovery"
	projectHubActionCategoryDanger   ListCategoryID = "project-hub-actions:danger"
)

const (
	projectHubItemPrefix         = "project-hub:project:"
	projectHubArchivedItemPrefix = "project-hub:archived:"
)

var projectHubActionCategories = []ListCategory{
	{ID: projectHubActionCategoryProject, Label: "Project"},
	{ID: projectHubActionCategoryRecovery, Label: "Recovery"},
	{ID: projectHubActionCategoryDanger, Label: "Danger"},
}

type projectHubEntryKind uint8

const (
	projectHubEntryActive projectHubEntryKind = iota
	projectHubEntryArchived
)

// projectHubTaskFilter is the Ctrl+T axis: every project, only the projects
// known to hold open tasks, or only the projects known to hold none open. It is
// one cycling axis rather than two independent toggles because the two
// narrowings are mutually exclusive readings of the same count.
type projectHubTaskFilter uint8

const (
	projectHubTaskFilterAll projectHubTaskFilter = iota
	projectHubTaskFilterWithOpenTasks
	projectHubTaskFilterWithoutOpenTasks
)

func (f projectHubTaskFilter) next() projectHubTaskFilter {
	switch f {
	case projectHubTaskFilterWithOpenTasks:
		return projectHubTaskFilterWithoutOpenTasks
	case projectHubTaskFilterWithoutOpenTasks:
		return projectHubTaskFilterAll
	default:
		return projectHubTaskFilterWithOpenTasks
	}
}

// shortLabel names the state for the footer hint, which reads as what the next
// press produces rather than as the state the axis currently holds.
func (f projectHubTaskFilter) shortLabel() string {
	switch f {
	case projectHubTaskFilterWithOpenTasks:
		return "with open tasks"
	case projectHubTaskFilterWithoutOpenTasks:
		return "without open tasks"
	default:
		return "all projects"
	}
}

// phrase names the state in prose, for the empty-list copy that has to explain
// where the next press lands.
func (f projectHubTaskFilter) phrase() string {
	switch f {
	case projectHubTaskFilterWithOpenTasks:
		return "projects that hold open tasks"
	case projectHubTaskFilterWithoutOpenTasks:
		return "projects without open tasks"
	default:
		return "every project"
	}
}

func (f projectHubTaskFilter) subtitleMarker() string {
	if f == projectHubTaskFilterAll {
		return ""
	}
	return f.shortLabel()
}

type projectHubEntry struct {
	Kind    projectHubEntryKind
	Project *models.Project
}

// projectTaskTally pairs the tasks a project holds with the open subset.
// Both travel together because a total alone reads as pending work when
// every task in it is finished.
type projectTaskTally struct {
	Open  int
	Total int
	// Tasks retains the tree the counts were read from. The count pass already
	// loads every cataloged project through the Outliner's cached loader and then
	// discards the record, so keeping the tasks costs nothing and is the only way
	// the preview pane can list work for a parked or archived project, whose Hub
	// entry is a rehydrated catalog record holding no tasks at all.
	Tasks []*models.Task
}

func projectTaskTallyOf(project *models.Project) projectTaskTally {
	return projectTaskTally{
		Open:  project.CountActiveTasks(),
		Total: project.CountTasks(),
		Tasks: project.Tasks,
	}
}

// projectHubActionTarget freezes project identity before an action sheet or
// direct project operation executes.
type projectHubActionTarget struct {
	ProjectID string
}

// ProjectHubModel is the browse-first surface for selecting and administering
// every active and archived project. The cursor is live on press one and `/`
// opens the query, so the Hub's project vocabulary answers immediately instead
// of typing into a search input.
type ProjectHubModel struct {
	BaseSearchableView
	runtime          *config.RuntimeState
	mode             ProjectHubMode
	taskCounts       map[string]projectTaskTally
	surface          *ListSurfaceModel
	actions          *ContextActionSheetModel
	registry         *OperationRegistry
	executor         *OperationExecutor
	dispatch         operationKeyDispatch
	focusedProjectID string
	notice           string
	taskFilter       projectHubTaskFilter
	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the Hub keeps its own one-key prefix.
	pendingTopChord bool
	preview         projectHubPreviewState
}
