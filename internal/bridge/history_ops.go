package bridge

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

type historyEntry struct {
	TransactionID string    `json:"transaction_id"`
	Description   string    `json:"description"`
	Projects      []string  `json:"projects"`
	At            time.Time `json:"at"`
	Undone        bool      `json:"undone"`
	seq           int
}

type historyResult struct {
	Entries  []historyEntry `json:"entries"`
	NextUndo string         `json:"next_undo"`
	NextRedo string         `json:"next_redo"`
}

func (b *Bridge) historyProjects() ([]*models.Project, error) {
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	state := runtime.State()
	return append(slices.Clone(state.AvailableProjects()), state.ArchivedProjects()...), nil
}

func (b *Bridge) queryHistory(request Request) (any, error) {
	if err := decodePayload(request.Payload, &struct{}{}); err != nil {
		return nil, err
	}
	projects, err := b.historyProjects()
	if err != nil {
		return nil, err
	}
	history, err := b.workspace.runtime.Ledger().History(storage.GlobalScope())
	if err != nil {
		return nil, err
	}
	result := historyResult{Entries: []historyEntry{}}
	if len(history.Undo) > 0 {
		result.NextUndo = history.Undo[0].TransactionID
	}
	if len(history.Redo) > 0 {
		result.NextRedo = history.Redo[0].TransactionID
	}
	names := map[string]string{}
	slugs := map[string]string{}
	for _, project := range projects {
		names[project.ID] = project.Name
		slugs[project.ID] = project.Slug
	}
	for _, stack := range []struct {
		entries []storage.LedgerEntry
		undone  bool
	}{{history.Undo, false}, {history.Redo, true}} {
		for _, entry := range stack.entries {
			labels := []string{}
			knownSlugs := map[string]bool{}
			for _, id := range entry.ProjectIDs {
				if name := names[id]; name != "" {
					labels = append(labels, name)
					knownSlugs[slugs[id]] = true
				}
			}
			// Deleted projects still have human-readable slugs in the retained paths.
			if len(labels) < len(entry.ProjectIDs) {
				for _, artifact := range entry.Artifacts {
					rel, err := filepath.Rel(b.workspace.runtime.DataDir(), artifact.Path)
					parts := strings.Split(filepath.ToSlash(rel), "/")
					if err == nil && len(parts) >= 2 && parts[0] == "projects" {
						slug := parts[1]
						if !knownSlugs[slug] && !slices.Contains(labels, slug) {
							labels = append(labels, slug)
						}
					}
				}
			}
			if len(labels) == 0 {
				labels = append(labels, "Workspace")
			}
			result.Entries = append(result.Entries, historyEntry{entry.TransactionID, entry.Description(), labels, entry.Timestamp, stack.undone, entry.Seq})
		}
	}
	slices.SortFunc(result.Entries, func(a, b historyEntry) int { return cmp.Compare(b.seq, a.seq) })
	return result, nil
}

func (b *Bridge) queryHistoryDetail(request Request) (any, error) {
	var payload struct {
		TransactionID string `json:"transaction_id"`
	}
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.TransactionID == "" {
		return nil, newError(CodeInvalidRequest, "transaction_id is required")
	}
	images, err := b.workspace.runtime.Ledger().HistoryArtifacts(payload.TransactionID)
	if err != nil {
		return nil, newError(CodeRefused, "Could not read this retained change: %v", err)
	}
	projects, err := b.historyProjects()
	if err != nil {
		return nil, err
	}
	type change struct {
		Entity string `json:"entity"`
		Field  string `json:"field"`
		Before string `json:"before"`
		After  string `json:"after"`
	}
	changes := []change{}
	for _, value := range app.ChangeHistoryDetails(images, projects) {
		changes = append(changes, change{value.Entity, value.Field, value.Before, value.After})
	}
	return struct {
		Changes []change `json:"changes"`
	}{changes}, nil
}
