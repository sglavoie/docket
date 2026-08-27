package tui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// noteSearchIndexOwner is the value-only owner description a background note
// read carries. The read runs off the update loop, so it is deliberately given
// no *models.Project: nothing it touches can be mutated underneath it.
type noteSearchIndexOwner struct {
	Location app.NoteLocation
	Label    string
}

// noteSearchIndexLoadedMsg carries one completed background note read. The
// generation discards a superseded result the same way crossProjectsLoadedMsg
// does, so a slow read can never overwrite a newer one.
type noteSearchIndexLoadedMsg struct {
	generation int
	rows       []LocatedNoteRow
	err        error
}

// noteSearchIndexCmd reads every requested note collection off the update loop.
// The Outliner's search tier ranks this index on every keystroke, so the disk
// read that produces it must happen exactly once per invalidation and never
// inside a row rebuild.
func noteSearchIndexCmd(
	service projectNoteService,
	generation int,
	owners []noteSearchIndexOwner,
) tea.Cmd {
	return func() tea.Msg {
		if service == nil {
			return noteSearchIndexLoadedMsg{generation: generation}
		}
		locations := make([]app.NoteLocation, 0, len(owners))
		labels := make(map[app.NoteLocation]string, len(owners))
		for _, owner := range owners {
			locations = append(locations, owner.Location)
			labels[owner.Location] = owner.Label
		}
		results, err := service.NotesAtLocations(locations)
		if err != nil {
			return noteSearchIndexLoadedMsg{generation: generation, err: err}
		}
		rows := make([]LocatedNoteRow, 0, len(results))
		for _, result := range results {
			// A collection that failed to read contributes nothing rather than
			// failing the whole index: the Notes manager is the surface that
			// reports a broken collection, and search is read-only discovery
			// that must keep working over the collections that did load.
			if result.Err != nil {
				continue
			}
			for _, note := range result.Notes {
				if note == nil {
					continue
				}
				rows = append(rows, LocatedNoteRow{
					Note:       note,
					Location:   result.Location,
					OwnerLabel: labels[result.Location],
				})
			}
		}
		sortNoteSearchIndexRows(rows)
		return noteSearchIndexLoadedMsg{generation: generation, rows: rows}
	}
}

// sortNoteSearchIndexRows gives the index one deterministic order — location,
// then filename — so a ranking tie resolves the same way on every rebuild.
func sortNoteSearchIndexRows(rows []LocatedNoteRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if order := compareNoteLocations(rows[i].Location, rows[j].Location); order != 0 {
			return order < 0
		}
		return rows[i].Note.Filename < rows[j].Note.Filename
	})
}

// noteSearchIndexOwners freezes the current owner catalog. Global leads, then
// every available project in catalog order, all of it admitted by the same
// noteOwnerCatalog rule the Notes manager uses, so an archived project is
// already absent here rather than filtered later.
func (m *AppModel) noteSearchIndexOwners() []noteSearchIndexOwner {
	global := noteSearchIndexOwner{Location: app.GlobalNoteLocation(), Label: "Global"}
	if m == nil || m.runtime == nil {
		return []noteSearchIndexOwner{global}
	}
	catalog, projects := noteOwnerCatalog(
		storage.GlobalNotesDir(m.runtime.DataDir()),
		m.availableProjects(),
	)
	owners := make([]noteSearchIndexOwner, 0, len(projects)+1)
	owners = append(owners, global)
	for _, project := range projects {
		location := app.ProjectNoteLocation(project.ID)
		owners = append(owners, noteSearchIndexOwner{
			Location: location,
			Label:    catalog[location].Label,
		})
	}
	return owners
}

func noteSearchIndexFingerprint(owners []noteSearchIndexOwner) string {
	parts := make([]string, 0, len(owners))
	for _, owner := range owners {
		parts = append(parts, string(owner.Location.Scope)+"\x00"+owner.Location.ProjectID+"\x00"+owner.Label)
	}
	return strings.Join(parts, "\n")
}

// requestNoteSearchIndexLoad starts a fresh background read and supersedes any
// read still in flight. Every invalidation channel — a committed note
// transaction, a note ledger restore, and a change in the available-project
// population — routes here rather than mutating the rows directly.
func (m *AppModel) requestNoteSearchIndexLoad() tea.Cmd {
	owners := m.noteSearchIndexOwners()
	m.noteIndexOwners = noteSearchIndexFingerprint(owners)
	m.noteIndexGeneration++
	// A typed nil *app.Service would satisfy the interface without satisfying
	// the nil check inside the command, so the seam is narrowed here.
	var service projectNoteService
	if m.appService != nil {
		service = m.appService
	}
	return noteSearchIndexCmd(service, m.noteIndexGeneration, owners)
}

// refreshNoteSearchIndexOwners reloads only when the owner catalog actually
// moved. A cross-project load lands after every committed task mutation, and
// re-reading every note collection from disk for a task edit would put a full
// notes read behind an ordinary keystroke.
func (m *AppModel) refreshNoteSearchIndexOwners() tea.Cmd {
	if noteSearchIndexFingerprint(m.noteSearchIndexOwners()) == m.noteIndexOwners {
		return nil
	}
	return m.requestNoteSearchIndexLoad()
}

func (m *AppModel) handleNoteSearchIndexLoaded(msg noteSearchIndexLoadedMsg) {
	if msg.generation != m.noteIndexGeneration {
		return
	}
	if msg.err != nil {
		// The retained rows stay live: a failed refresh degrades the tier to a
		// stale answer, which is strictly better than an empty one.
		logging.Warn("failed to load the Outliner note search index: %v", msg.err)
		return
	}
	m.noteIndexRows = msg.rows
}

// noteSearchIndexRows is the accessor the Outliner is injected with. It is a
// method value rather than a copied slice so an invalidation that lands while a
// search projection is open is visible to the very next rebuild.
func (m *AppModel) noteSearchIndexRows() []LocatedNoteRow {
	return m.noteIndexRows
}
