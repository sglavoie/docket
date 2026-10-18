package outliner

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// A request is owned by the update loop. The command reads only its detached
// model and context; it never reads this flight or the live Outliner.
type backgroundSearch struct {
	run         tea.Cmd
	cancel      context.CancelFunc
	revision    uint64
	fingerprint string
	anchor      modeCursorAnchor
	result      *OutlinerModel
}

type BackgroundSearchResultMsg struct {
	flight *backgroundSearch
	result *OutlinerModel
}

// Immutable inputs are reused while only the typed query changes. This bounds
// retention to one workspace snapshot instead of cloning 10,000 tasks per key.
type backgroundSearchSnapshot struct {
	key      string
	revision uint64
	model    *OutlinerModel
	live     map[taskref.TaskIdentity]*models.Task
}

func (s *searchState) cancelBackground() {
	if s.flight != nil {
		s.flight.cancel()
		s.flight = nil
	}
}

func (m *OutlinerModel) takeSearchCommand() tea.Cmd {
	cmd := m.Search.queued
	m.Search.queued = nil
	return cmd
}

// Small searches stay synchronous. Bound by both population and text volume so
// a handful of large descriptions/notes also leave the input loop promptly.
func (m *OutlinerModel) needsBackgroundSearch() bool {
	if !m.ShowsRankedList() || m.SearchQuery() == "" {
		return false
	}
	count, size := 0, 0
	for _, project := range m.WorkModeScopeProjects() {
		if project == nil {
			continue
		}
		count += len(project.Tasks)
		if count >= 1000 {
			return true
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			size += len(task.Title) + len(task.Description) + len(task.ResumeNote)
			for _, comment := range task.Comments {
				if comment != nil {
					size += len(comment.Text)
				}
			}
			for _, item := range task.Checklist {
				size += len(item.Text)
			}
			if size >= 1<<20 {
				return true
			}
		}
	}
	for _, row := range m.noteSearchRowsInScope() {
		count++
		if row.Note != nil {
			size += len(row.Note.Title) + len(row.Note.Body)
		}
		if count >= 1000 || size >= 1<<20 {
			return true
		}
	}
	return false
}

func (m *OutlinerModel) rebuildLiveSearch() tea.Cmd {
	anchor := m.captureModeCursor()
	var reusableRows []OutlinerRow
	if m.Search.flight != nil && m.Search.flight.result != nil {
		reusableRows = m.Search.flight.result.Rows[:0]
	}
	if m.Search.flight != nil && m.Search.flight.result == nil {
		anchor = m.Search.flight.anchor
	}
	m.Search.cancelBackground()
	if !m.needsBackgroundSearch() {
		RebuildOutlinerPreservingCursor(m)
		return nil
	}
	worker, err := m.detachedSearchModel()
	if err != nil {
		// Keep the query editable and report the failure; never hide a failed
		// snapshot behind a costly synchronous fallback.
		m.Rows = nil
		return status.ErrorCmd("Search could not start: %v", err)
	}
	worker.Rows = reusableRows
	ctx, cancel := context.WithCancel(context.Background())
	worker.Search.workContext = ctx
	flight := &backgroundSearch{cancel: cancel, revision: m.Population.revision,
		fingerprint: m.crossProjectRowsFingerprint(), anchor: anchor}
	m.Search.flight = flight
	if m.Search.worker == nil {
		m.Search.worker = &sync.Mutex{}
	}
	serial := m.Search.worker
	m.rebuildRows()
	flight.run = func() tea.Msg {
		serial.Lock()
		defer serial.Unlock()
		if ctx.Err() != nil {
			return nil
		}
		worker.buildSearchProjectionRows()
		if ctx.Err() != nil {
			return nil
		}
		return BackgroundSearchResultMsg{flight: flight, result: worker}
	}
	return flight.run
}

// Detach every mutable input, including planning, project metadata, comments,
// and note bodies. The scorer and all empty-state diagnostics stay shared with
// synchronous search; no second query language or ranking path exists.
func (m *OutlinerModel) detachedSearchModel() (*OutlinerModel, error) {
	key := fmt.Sprintf("%p|%s|%v|%v", m.Project, m.crossProjectRowsFingerprintForSearch(""), m.Focus.Stack, m.Focus.CrossStack)
	snapshot := m.Search.snapshot
	if snapshot == nil || snapshot.key != key || snapshot.revision != m.Population.revision {
		base, err := m.cloneSearchInputs()
		if err != nil {
			return nil, err
		}
		live := make(map[taskref.TaskIdentity]*models.Task)
		for _, project := range m.WorkModeScopeProjects() {
			if project == nil {
				continue
			}
			for _, task := range project.Tasks {
				if task != nil {
					live[taskref.For(task, project)] = task
				}
			}
		}
		snapshot = &backgroundSearchSnapshot{key: key, revision: m.Population.revision, model: base, live: live}
		m.Search.snapshot = snapshot
	}
	worker := *snapshot.model
	worker.BaseSearchableView.SetSearchQuery(m.SearchQuery())
	worker.Search = searchState{
		Projection: &outlinerSearchSnapshot{}, rankedList: true,
		scoreCache: m.searchScoreCache(), noteIndex: snapshot.model.Search.noteIndex,
	}
	return &worker, nil
}

func (m *OutlinerModel) cloneSearchInputs() (*OutlinerModel, error) {
	worker := &OutlinerModel{PrimaryMode: m.PrimaryMode, Sort: m.Sort}
	if m.Runtime != nil {
		domains, err := m.Runtime.CloneDomains()
		if err != nil {
			return nil, err
		}
		worker.Runtime = config.NewRuntimeState("", domains)
	}
	cloneProject := func(project *models.Project) *models.Project {
		if project == nil {
			return nil
		}
		clone := *project
		clone.Tags = slices.Clone(project.Tags)
		clone.ArchivedAt = models.CloneInstant(project.ArchivedAt)
		clone.PinnedAt = models.CloneInstant(project.PinnedAt)
		clone.Comments = models.CloneProjectComments(project.Comments)
		clone.Tasks = make([]*models.Task, len(project.Tasks))
		for i, task := range project.Tasks {
			clone.Tasks[i] = models.CloneTaskDeep(task)
		}
		clone.BuildTaskTree()
		return &clone
	}
	for _, project := range m.Population.Projects {
		worker.Population.Projects = append(worker.Population.Projects, cloneProject(project))
	}
	worker.Project = cloneProject(m.Project)
	worker.Scope.ProjectID = m.Scope.ProjectID
	worker.Focus.Restore(slices.Clone(m.Focus.Stack), slices.Clone(m.Focus.CrossStack), m.Focus.Zoom)
	worker.SetFilterQuery(m.FilterQuery)
	rows := slices.Clone(m.noteSearchRowsInScope())
	for i := range rows {
		if rows[i].Note != nil {
			note := *rows[i].Note
			rows[i].Note = &note
		}
	}
	worker.Search.noteIndex = func() []notes.LocatedNoteRow { return rows }
	worker.openChildCounts = query.OpenChildCounts(worker.WorkModeScopeProjects())
	worker.timeDrivenDay = time.Now()
	return worker, nil
}

func (m *OutlinerModel) backgroundCurrent(flight *backgroundSearch) bool {
	return flight != nil && m.Search.flight == flight && m.ShowsRankedList() &&
		flight.revision == m.Population.revision && flight.fingerprint == m.crossProjectRowsFingerprint()
}

func (m *OutlinerModel) acceptBackgroundSearch(msg BackgroundSearchResultMsg) {
	if !m.backgroundCurrent(msg.flight) {
		return
	}
	// Rows name live task objects only after the snapshot generation is accepted.
	// Editors and operation targets must never receive the worker's copies.
	live := m.Search.snapshot.live
	for i := range msg.result.Rows {
		row := &msg.result.Rows[i]
		if row.Task == nil {
			continue
		}
		projectID := row.ProjectID
		if m.Project != nil {
			projectID = m.Project.ID
		}
		row.Task = live[taskref.TaskIdentity{ProjectID: projectID, TaskID: row.Task.ID}]
	}
	msg.flight.result = msg.result
	m.rebuildRows()
	m.restoreModeCursor(msg.flight.anchor)
	m.ClampCursor()
}

func (m *OutlinerModel) useBackgroundRows() bool {
	flight := m.Search.flight
	if flight == nil {
		return false
	}
	if !m.backgroundCurrent(flight) {
		m.Search.cancelBackground()
		return false
	}
	m.Search.clearNoteMatches()
	m.Search.withheld = searchWithheldMatches{}
	m.Search.hiddenFilterHint, m.Search.eliminationHint = "", ""
	if flight.result == nil {
		m.Search.setCompilation("", nil, false)
		return true
	}
	result := flight.result
	m.Rows = append(m.Rows, result.Rows...)
	m.Search.setCompilation(result.Search.remainder, result.Search.Gate, result.Search.caseRelaxed)
	m.Search.withheld = result.Search.withheld
	m.Search.hiddenFilterHint, m.Search.eliminationHint = result.Search.hiddenFilterHint, result.Search.eliminationHint
	m.Search.noteMatches = result.Search.noteMatches
	return true
}

func (m *OutlinerModel) searchContext() context.Context {
	if m.Search.workContext != nil {
		return m.Search.workContext
	}
	return context.Background()
}
