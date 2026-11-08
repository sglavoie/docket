package help

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
)

// CommandPaletteModel is `:` on the Outliner: help search's runnable action
// list as a modal over the view it was opened from. It owns no catalog and no
// invocation of its own. The shell hands it the same items Outliner help search
// receives and the context frozen at the keypress, and a chosen row becomes the
// same execution request a help result sends, so the executor and its
// refusals are the key's own.
//
// The input is focused from the first keystroke, which is the difference from
// help: there is no browse grammar to leave, so every key the palette does not
// name for moving or choosing is text.
type CommandPaletteModel struct {
	context    operation.CommandContext
	items      []operation.HelpItem
	categories []string
	query      string
	input      listsurface.PickerSearchInput
	results    []filteredHelpItem
	cursor     int
	// top is the first result the last render drew, so moving the cursor scrolls
	// the window by whole rows rather than jumping it back to the start.
	top int
	// pageRows is the result window the last render drew, which is what a page
	// key travels.
	pageRows int
	// recent ranks the operation IDs this machine last ran from the palette,
	// most recent first, ahead of the footer ladder on an empty query.
	recent map[string]int
}

// PaletteRecentKey is how a palette row is remembered: its operation, and its
// preset value where it carries one, since Complete Task and Wait on Task are
// one operation told apart only by the state they pass.
func PaletteRecentKey(id operation.OperationID, value any, hasValue bool) string {
	if !hasValue {
		return string(id)
	}
	return string(id) + "=" + fmt.Sprint(value)
}

func paletteItemKey(item operation.HelpItem) string {
	if item.Action == nil {
		return ""
	}
	return PaletteRecentKey(item.Action.ID, item.Action.Value, item.Action.HasValue)
}

// RunPaletteActionMsg asks the shell to close this palette and execute the
// chosen registry action against the context frozen when it opened.
type RunPaletteActionMsg struct {
	Source  *CommandPaletteModel
	Request operation.OperationExecutionRequest
}

// NewCommandPalette keeps the runnable rows alone: reference prose has nothing
// to run, and the palette does not list itself.
func NewCommandPalette(ctx operation.CommandContext, items []operation.HelpItem) *CommandPaletteModel {
	runnable := make([]operation.HelpItem, 0, len(items))
	categories := make([]string, 0, len(items))
	for _, item := range items {
		if item.Action == nil || item.Action.ID == operation.OperationCoreCommandPalette {
			continue
		}
		runnable = append(runnable, item)
		categories = append(categories, helpSectionForItem(item))
	}
	p := &CommandPaletteModel{
		context:    operation.CloneCommandContext(ctx),
		items:      runnable,
		categories: helpSectionsForCategories(categories),
	}
	p.filter()
	return p
}

// SetRecent names the actions last run from the palette, most recent first, and
// re-reads the results so an empty query opens on them.
func (p *CommandPaletteModel) SetRecent(keys []string) {
	if p == nil {
		return
	}
	p.recent = make(map[string]int, len(keys))
	for index, key := range keys {
		if _, seen := p.recent[key]; !seen {
			p.recent[key] = index
		}
	}
	p.filter()
}

// IsRecent reports whether the item is one the palette leads with as recent.
func (p *CommandPaletteModel) IsRecent(item operation.HelpItem) bool {
	if p == nil || item.Action == nil {
		return false
	}
	_, ok := p.recent[paletteItemKey(item)]
	return ok
}

func (p *CommandPaletteModel) recentRank(item operation.HelpItem) (int, bool) {
	if item.Action == nil || item.DisabledReason != "" {
		return 0, false
	}
	rank, ok := p.recent[paletteItemKey(item)]
	return rank, ok
}

// filter re-reads the results for the current query. An empty query lists every
// action with the available ones first: those this machine ran most recently
// lead, then the rest in footer-rank order, so the head of the list is what
// this person reaches for and then the head of the footer ladder. A typed query
// is matched and ranked exactly as help search ranks it, with recency breaking
// only exact ties.
func (p *CommandPaletteModel) filter() {
	p.cursor, p.top = 0, 0
	if strings.TrimSpace(p.query) == "" {
		p.results = make([]filteredHelpItem, len(p.items))
		for i, item := range p.items {
			p.results[i] = filteredHelpItem{HelpItem: item}
		}
		sort.SliceStable(p.results, func(i, j int) bool {
			a, b := p.results[i], p.results[j]
			ra, aRecent := p.recentRank(a.HelpItem)
			rb, bRecent := p.recentRank(b.HelpItem)
			if aRecent != bRecent {
				return aRecent
			}
			if aRecent {
				return ra < rb
			}
			if (a.DisabledReason == "") != (b.DisabledReason == "") {
				return a.DisabledReason == ""
			}
			if (a.Rank > 0) != (b.Rank > 0) {
				return a.Rank > 0
			}
			return a.Rank < b.Rank
		})
		return
	}
	p.results, _ = matchHelpItems(p.items, p.query)
	rankActionResults(p.results, p.query, p.categories)
}

// Selected is the result under the cursor.
func (p *CommandPaletteModel) Selected() (operation.HelpItem, bool) {
	if p == nil || len(p.results) == 0 {
		return operation.HelpItem{}, false
	}
	return p.results[min(max(p.cursor, 0), len(p.results)-1)].HelpItem, true
}

// HandleKey answers every key while the palette is mounted. Enter on an
// unavailable row runs nothing: the reason is already on screen under the list.
func (p *CommandPaletteModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if p == nil {
		return nil, true, nil
	}
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		item, ok := p.Selected()
		if !ok || item.DisabledReason != "" {
			return nil, false, nil
		}
		return nil, false, RunPaletteActionMsg{Source: p, Request: operation.OperationExecutionRequest{
			ID: item.Action.ID, Value: item.Action.Value, HasValue: item.Action.HasValue,
			Context: operation.CloneCommandContext(p.context),
		}}
	case "up", "ctrl+p", "ctrl+k":
		p.move(-1)
	case "down", "ctrl+n", "ctrl+j":
		p.move(1)
	case "pgup":
		p.move(-max(p.pageRows-1, 1))
	case "pgdown":
		p.move(max(p.pageRows-1, 1))
	default:
		if edited := p.input.HandleKey(p.query, msg); edited != p.query {
			p.query = edited
			p.filter()
		}
	}
	return nil, false, nil
}

// HandlePaste types a bracketed paste into the query, one line of it, since an
// action name never spans two.
func (p *CommandPaletteModel) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if p == nil {
		return nil
	}
	if text := strings.Join(strings.Fields(msg.Content), " "); text != "" {
		p.query += text
		p.filter()
	}
	return nil
}

func (p *CommandPaletteModel) move(delta int) {
	if len(p.results) == 0 {
		return
	}
	p.cursor = max(0, min(p.cursor+delta, len(p.results)-1))
}

// RefuseAction keeps the palette open when the target changed after it opened,
// and keeps the explanation on that row until it is reopened.
func (p *CommandPaletteModel) RefuseAction(id operation.OperationID, reason string) {
	if p == nil {
		return
	}
	for i, item := range p.items {
		if item.Action.ID == id {
			p.items[i].DisabledReason = reason
		}
	}
	for i, item := range p.results {
		if item.Action.ID == id {
			p.results[i].DisabledReason = reason
		}
	}
}
