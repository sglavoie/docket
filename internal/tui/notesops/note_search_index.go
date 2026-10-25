package notesops

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/notes"
)

// noteSearchIndexOwner is the value-only owner description a background note
// read carries. The read runs off the update loop, so it is deliberately given
// no *models.Project: nothing it touches can be mutated underneath it.
type noteSearchIndexOwner struct {
	Location app.NoteLocation
	Label    string
}

// NoteSearchIndexLoadedMsg carries one completed background note read. The
// generation discards a superseded result the same way crossProjectsLoadedMsg
// does, so a slow read can never overwrite a newer one.
type NoteSearchIndexLoadedMsg struct {
	generation int
	rows       []notes.LocatedNoteRow
	err        error
}

// noteSearchIndexCmd reads every requested note collection off the update loop.
// The Outliner's search tier ranks this index on every keystroke, so the disk
// read that produces it must happen exactly once per invalidation and never
// inside a row rebuild.
func noteSearchIndexCmd(
	service notes.ProjectNoteService,
	generation int,
	owners []noteSearchIndexOwner,
) tea.Cmd {
	return func() tea.Msg {
		if service == nil {
			return NoteSearchIndexLoadedMsg{generation: generation}
		}
		locations := make([]app.NoteLocation, 0, len(owners))
		labels := make(map[app.NoteLocation]string, len(owners))
		for _, owner := range owners {
			locations = append(locations, owner.Location)
			labels[owner.Location] = owner.Label
		}
		results, err := service.NotesAtLocations(locations)
		if err != nil {
			return NoteSearchIndexLoadedMsg{generation: generation, err: err}
		}
		rows := make([]notes.LocatedNoteRow, 0, len(results))
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
				rows = append(rows, notes.LocatedNoteRow{
					Note:       note,
					Location:   result.Location,
					OwnerLabel: labels[result.Location],
				})
			}
		}
		notes.SortLocatedNoteRows(rows)
		return NoteSearchIndexLoadedMsg{generation: generation, rows: rows}
	}
}

// noteSearchIndexOwners freezes the current owner catalog. Global leads, then
// every available project in catalog order, all of it admitted by the same
// NoteOwnerCatalog rule the Notes manager uses, so an archived project is
// already absent here rather than filtered later.
func (c *Controller) noteSearchIndexOwners(shell Shell) []noteSearchIndexOwner {
	global := noteSearchIndexOwner{Location: app.GlobalNoteLocation(), Label: "Global"}
	if shell == nil || shell.DomainRuntime() == nil {
		return []noteSearchIndexOwner{global}
	}
	catalog, projects := notes.NoteOwnerCatalog(
		storage.GlobalNotesDir(shell.DomainRuntime().DataDir()),
		shell.AvailableProjects(),
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

// RequestNoteSearchIndexLoad starts a fresh background read and supersedes any
// read still in flight. Every invalidation channel — a committed note
// transaction, a note ledger restore, and a change in the available-project
// population — routes here rather than mutating the rows directly.
func (c *Controller) RequestNoteSearchIndexLoad(shell Shell) tea.Cmd {
	owners := c.noteSearchIndexOwners(shell)
	generation := c.index.beginLoad(noteSearchIndexFingerprint(owners))
	// A typed nil *app.Service would satisfy the interface without satisfying
	// the nil check inside the command, so the seam is narrowed here.
	var service notes.ProjectNoteService
	if shell.DomainService() != nil {
		service = shell.DomainService()
	}
	return noteSearchIndexCmd(service, generation, owners)
}

// RefreshNoteSearchIndexOwners reloads only when the owner catalog actually
// moved. A cross-project load lands after every committed task mutation, and
// re-reading every note collection from disk for a task edit would put a full
// notes read behind an ordinary keystroke.
func (c *Controller) RefreshNoteSearchIndexOwners(shell Shell) tea.Cmd {
	if !c.index.ownersMoved(noteSearchIndexFingerprint(c.noteSearchIndexOwners(shell))) {
		return nil
	}
	return c.RequestNoteSearchIndexLoad(shell)
}

func (c *Controller) HandleNoteSearchIndexLoaded(shell Shell, msg NoteSearchIndexLoadedMsg) {
	if !c.index.awaits(msg.generation) {
		return
	}
	if msg.err != nil {
		// The retained rows stay live: a failed refresh degrades the tier to a
		// stale answer, which is strictly better than an empty one.
		logging.Warn("failed to load the Outliner note search index: %v", msg.err)
		return
	}
	c.index.adopt(msg.rows)
	c.invalidateOutlinerForNoteSearchIndex(shell)
}

// invalidateOutlinerForNoteSearchIndex forces the authoritative Outliner to
// rebuild after an accepted index result. The note index is read through an
// injected accessor rather than held in the Outliner, so it is invisible to
// crossProjectRowsFingerprint: without this the memoization guard would
// short-circuit the next render and an already-open search would keep showing
// the notes tier the superseded index produced. Only an accepted result reaches
// here, so a stale generation or a failed read still leaves the displayed
// projection exactly as it was.
//
// The cursor's own note identity is captured alongside the flag: the rebuild
// re-ranks the tier, so a refreshed or newly matching note can move a still
// present result and the retained numeric row would then name a different one.
func (c *Controller) invalidateOutlinerForNoteSearchIndex(shell Shell) {
	if shell == nil || shell.Outliner() == nil {
		return
	}
	if identity, onNote := shell.Outliner().CursorNoteIdentity(); onNote {
		shell.Outliner().Search.CaptureNoteCursor(identity)
	}
	shell.Outliner().Population.Invalidate()
}

// NoteSearchIndexRows is the accessor the Outliner is injected with. It is a
// method value rather than a copied slice so an invalidation that lands while a
// search projection is open is visible to the very next rebuild.
func (c *Controller) NoteSearchIndexRows(shell Shell) []notes.LocatedNoteRow {
	return c.index.rows
}
