package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// HistoryChange describes the original transaction, before → after. It is not
// a prediction of restore success: the restore engine rechecks current data.
type HistoryChange struct {
	Entity, Field, Before, After string
}

// ChangeHistoryDetails interprets retained images, never live artifact bytes.
// Projects supplies display names for references missing from those images.
func ChangeHistoryDetails(images []storage.LedgerArtifact, projects []*models.Project) []HistoryChange {
	names := historyNames{projects: map[string]string{}, tasks: map[config.ProjectTaskLocator]string{}}
	for _, p := range projects {
		names.projects[p.ID] = p.Slug
		for _, task := range p.Tasks {
			names.tasks[config.ProjectTaskLocator{ProjectID: p.ID, TaskID: task.ID}] = task.Title
		}
	}
	// Catalog snapshots also name archived/deleted projects. Retained task
	// images supply titles even when the current view never loaded that project.
	for _, image := range images {
		if historyDomainFile(image.Path, "workspace.json") {
			for _, data := range [][]byte{image.Before, image.After} {
				var catalog config.WorkspaceCatalog
				if historyDomain(data, &catalog) == nil {
					for _, p := range catalog.Projects {
						names.projects[p.ID] = p.Slug
					}
				}
			}
		}
	}
	for _, image := range images {
		if !historyTaskDocument(image.Path) {
			continue
		}
		slug := filepath.Base(filepath.Dir(image.Path))
		for _, data := range [][]byte{image.Before, image.After} {
			if doc, err := storage.ParseProjectDocument(string(data)); err == nil {
				for id, projectSlug := range names.projects {
					if projectSlug == slug {
						for _, task := range doc.Tasks {
							names.tasks[config.ProjectTaskLocator{ProjectID: id, TaskID: task.ID}] = task.Title
						}
					}
				}
			}
		}
	}
	var changes []HistoryChange
	for _, image := range images {
		switch {
		case historyTaskDocument(image.Path):
			changes = append(changes, historyProjectChanges(image, names)...)
		case historyDomainFile(image.Path, "planning.json"):
			changes = append(changes, historyPlanningChanges(image, names)...)
		case historyDomainFile(image.Path, "workspace.json"):
			changes = append(changes, historyCatalogChanges(image)...)
		default:
			changes = append(changes, historyArtifactChange(image))
		}
	}
	return changes
}

type historyNames struct {
	projects map[string]string
	tasks    map[config.ProjectTaskLocator]string
}

func (n historyNames) task(ref config.ProjectTaskLocator) string {
	project := n.projects[ref.ProjectID]
	if project == "" {
		project = ref.ProjectID
	}
	title := n.tasks[ref]
	if title == "" {
		title = ref.TaskID
	}
	return "[" + project + "] " + title
}

func historyDomainFile(path, name string) bool {
	return filepath.Base(path) == name && filepath.Base(filepath.Dir(path)) == ".docket"
}

func historyTaskDocument(path string) bool {
	return filepath.Base(path) == "tasks.md" && filepath.Base(filepath.Dir(filepath.Dir(path))) == "projects"
}

func historyProjectChanges(image storage.LedgerArtifact, names historyNames) []HistoryChange {
	before, beforeErr := storage.ParseProjectDocument(string(image.Before))
	after, afterErr := storage.ParseProjectDocument(string(image.After))
	if (!image.BeforeExists || beforeErr == nil) && (!image.AfterExists || afterErr == nil) {
		if !image.BeforeExists {
			before = &storage.ProjectDocument{}
		}
		if !image.AfterExists {
			after = &storage.ProjectDocument{}
		}
	} else {
		return []HistoryChange{{Entity: historyArtifactLabel(image.Path), Field: "Details", Before: "", After: "Retained task document cannot be read by this version."}}
	}
	project := filepath.Base(filepath.Dir(image.Path))
	var changes []HistoryChange
	add := func(entity, field, a, b string) {
		if a != b {
			changes = append(changes, HistoryChange{entity, field, a, b})
		}
	}
	add("["+project+"]", "Project name", before.ProjectName, after.ProjectName)
	old, next := map[string]*models.Task{}, map[string]*models.Task{}
	var ids, oldOrder, newOrder []string
	for _, task := range before.Tasks {
		old[task.ID] = task
		ids = append(ids, task.ID)
		oldOrder = append(oldOrder, task.ID)
	}
	for _, task := range after.Tasks {
		next[task.ID] = task
		newOrder = append(newOrder, task.ID)
		if old[task.ID] == nil {
			ids = append(ids, task.ID)
		}
	}
	for _, id := range ids {
		a, b := old[id], next[id]
		if a == nil {
			add("["+project+"] "+b.Title, "Task", "Absent", "Created")
			for _, value := range historyTaskValues(b, next, names) {
				if value.Before != "" && value.Before != "None" && value.Before != "No" && value.Before != "0" {
					add("["+project+"] "+b.Title, value.Field, "", value.Before)
				}
			}
			continue
		}
		if b == nil {
			add("["+project+"] "+a.Title, "Task", "Present", "Deleted")
			for _, value := range historyTaskValues(a, old, names) {
				if value.Before != "" && value.Before != "None" && value.Before != "No" && value.Before != "0" {
					add("["+project+"] "+a.Title, value.Field, value.Before, "")
				}
			}
			continue
		}
		entity := "[" + project + "] " + b.Title
		av, bv := historyTaskValues(a, old, names), historyTaskValues(b, next, names)
		for i, value := range av {
			add(entity, value.Field, value.Before, bv[i].Before)
		}
	}
	if len(oldOrder) == len(newOrder) && !slices.Equal(oldOrder, newOrder) {
		ordered := func(tasks []*models.Task) string {
			var titles []string
			for _, task := range tasks {
				titles = append(titles, task.Title)
			}
			return strings.Join(titles, "\n")
		}
		add("["+project+"]", "Task order", ordered(before.Tasks), ordered(after.Tasks))
	}
	add("["+project+"]", "Project comments", historyProjectComments(before.Comments), historyProjectComments(after.Comments))
	return changes
}

func historyTaskValues(task *models.Task, tasks map[string]*models.Task, names historyNames) []HistoryChange {
	parent := "None"
	if task.ParentID != nil {
		parent = *task.ParentID
		if p := tasks[parent]; p != nil {
			parent = p.Title
		}
	}
	var checklist, comments, attachments, references []string
	for _, item := range task.Checklist {
		mark := "[ ] "
		if item.Done {
			mark = "[x] "
		}
		checklist = append(checklist, mark+item.Text)
	}
	for _, comment := range task.Comments {
		if comment != nil {
			comments = append(comments, comment.Text)
		}
	}
	for _, attachment := range task.Attachments {
		label := attachment.Filename
		if attachment.OriginalPath != "" && attachment.OriginalPath != label {
			label += " · " + attachment.OriginalPath
		}
		if attachment.LineNumber != 0 {
			label += fmt.Sprintf(":%d", attachment.LineNumber)
		}
		attachments = append(attachments, label)
	}
	for _, ref := range task.Dependencies {
		references = append(references, names.task(config.ProjectTaskLocator{ProjectID: ref.ProjectID, TaskID: ref.TaskID}))
	}
	values := []struct{ label, value string }{
		{"Title", task.Title}, {"Status", historyTitle(string(task.WorkflowState))},
		{"Description", task.Description}, {"Priority", strconv.Itoa(task.Priority)},
		{"Parent", parent}, {"Context", task.Context}, {"Tags", strings.Join(task.Tags, ", ")},
		{"Pinned", historyBool(task.Pinned)}, {"Review", historyBool(task.Review)}, {"Hidden", historyBool(task.Hidden)},
		{"Due", historyDue(task)}, {"Deferred until", historyInstant(task.DeferredUntil)},
		{"Deferral count", strconv.Itoa(task.DeferCount)},
		{"Check-in", historyDate(task.NextCheckIn)}, {"Check-in repeat (days)", strconv.Itoa(task.CheckInRepeatDays)},
		{"Last reviewed", historyInstant(task.LastReviewedAt)}, {"Resume note", task.ResumeNote},
		{"Checklist", strings.Join(checklist, "\n")}, {"Comments", strings.Join(comments, "\n\n")},
		{"Attachments", strings.Join(attachments, "\n")}, {"Dependencies", strings.Join(references, "\n")},
	}
	result := make([]HistoryChange, 0, len(values))
	for _, value := range values {
		result = append(result, HistoryChange{Field: value.label, Before: value.value})
	}
	return result
}

func historyPlanningChanges(image storage.LedgerArtifact, names historyNames) []HistoryChange {
	var a, b config.PlanningState
	if (image.BeforeExists && historyDomain(image.Before, &a) != nil) || (image.AfterExists && historyDomain(image.After, &b) != nil) {
		return []HistoryChange{historyArtifactChange(image)}
	}
	var changes []HistoryChange
	for _, lane := range []struct {
		label string
		a, b  []config.ProjectTaskLocator
	}{{"Today", a.Today, b.Today}, {"Next", a.Next, b.Next}} {
		for _, ref := range lane.a {
			if !slices.Contains(lane.b, ref) {
				changes = append(changes, HistoryChange{names.task(ref), lane.label, "Included", "Removed"})
			}
		}
		for _, ref := range lane.b {
			if !slices.Contains(lane.a, ref) {
				changes = append(changes, HistoryChange{names.task(ref), lane.label, "Absent", "Added"})
			}
		}
		if len(lane.a) == len(lane.b) && !slices.Equal(lane.a, lane.b) {
			allSame := true
			for _, ref := range lane.a {
				allSame = allSame && slices.Contains(lane.b, ref)
			}
			if allSame {
				labels := func(refs []config.ProjectTaskLocator) string {
					var out []string
					for _, ref := range refs {
						out = append(out, names.task(ref))
					}
					return strings.Join(out, "\n")
				}
				changes = append(changes, HistoryChange{"Planning", lane.label + " order", labels(lane.a), labels(lane.b)})
			}
		}
	}
	return changes
}

func historyCatalogChanges(image storage.LedgerArtifact) []HistoryChange {
	var a, b config.WorkspaceCatalog
	if (image.BeforeExists && historyDomain(image.Before, &a) != nil) || (image.AfterExists && historyDomain(image.After, &b) != nil) {
		return []HistoryChange{historyArtifactChange(image)}
	}
	old, next := map[string]config.CatalogProject{}, map[string]config.CatalogProject{}
	var ids []string
	for _, p := range a.Projects {
		old[p.ID] = p
		ids = append(ids, p.ID)
	}
	for _, p := range b.Projects {
		next[p.ID] = p
		if _, ok := old[p.ID]; !ok {
			ids = append(ids, p.ID)
		}
	}
	var changes []HistoryChange
	for _, id := range ids {
		p, was := old[id]
		q, now := next[id]
		if !was {
			changes = append(changes, HistoryChange{q.Name, "Project", "Absent", "Created"})
			continue
		}
		if !now {
			changes = append(changes, HistoryChange{p.Name, "Project", "Present", "Deleted"})
			continue
		}
		for _, field := range []struct{ label, a, b string }{
			{"Name", p.Name, q.Name}, {"Slug", p.Slug, q.Slug}, {"Description", p.Description, q.Description},
			{"Archived", historyBool(p.Archived), historyBool(q.Archived)}, {"Pinned", historyBool(p.Pinned), historyBool(q.Pinned)},
			{"Tags", strings.Join(p.Tags, ", "), strings.Join(q.Tags, ", ")}, {"Color", strconv.Itoa(p.ColorSlot), strconv.Itoa(q.ColorSlot)},
		} {
			if field.a != field.b {
				changes = append(changes, HistoryChange{q.Name, field.label, field.a, field.b})
			}
		}
	}
	return changes
}

func historyDomain(data []byte, target any) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Data) == 0 {
		return fmt.Errorf("missing retained domain payload")
	}
	return json.Unmarshal(envelope.Data, target)
}

func historyArtifactLabel(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts {
		if part == "projects" && i+1 < len(parts) {
			return strings.Join(parts[i+1:], "/")
		}
	}
	return filepath.Base(path)
}

func historyArtifactChange(image storage.LedgerArtifact) HistoryChange {
	value := func(exists bool, data []byte) string {
		if !exists {
			return "Absent"
		}
		if image.Kind == storage.ArtifactImageKindDirectory {
			return "Folder present"
		}
		if utf8.Valid(data) && !strings.ContainsRune(string(data), 0) && (strings.HasSuffix(image.Path, ".md") || strings.HasSuffix(image.Path, ".txt")) {
			return string(data)
		}
		return fmt.Sprintf("File present (%d bytes)", len(data))
	}
	before, after := value(image.BeforeExists, image.Before), value(image.AfterExists, image.After)
	if before == after && string(image.Before) != string(image.After) {
		after += " · contents changed"
	}
	return HistoryChange{historyArtifactLabel(image.Path), "Content", before, after}
}

func historyProjectComments(comments []*models.ProjectComment) string {
	var text []string
	for _, comment := range comments {
		if comment != nil {
			text = append(text, comment.Text)
		}
	}
	return strings.Join(text, "\n\n")
}

func historyBool(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}
func historyDue(task *models.Task) string {
	if task.DueHasTime() {
		return task.DueDate.Format("2006-01-02 15:04 MST")
	}
	return historyDate(task.DueDate)
}

func historyDate(value *time.Time) string {
	if value == nil {
		return "None"
	}
	return value.Format(time.DateOnly)
}
func historyInstant(value *time.Time) string {
	if value == nil {
		return "None"
	}
	return value.Local().Format("2006-01-02 15:04:05 MST")
}
func historyTitle(value string) string {
	if value == "" {
		return "None"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
