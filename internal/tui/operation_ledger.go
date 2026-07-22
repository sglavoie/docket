package tui

import (
	"sort"
	"strings"
	"unicode"
)

// TUIScenarioBaseline records the committed consolidated scenario inventory.
const TUIScenarioBaseline = 41

// OperationID is the stable semantic identity of a registered command.
type OperationID string

// CommandReachability records how an operation remains reachable in the final
// TUI, or why it is intentionally absent.
type CommandReachability string

const (
	CommandReachGlobalBinding CommandReachability = "global-binding"
	CommandReachViewMenu      CommandReachability = "view-menu"
	CommandReachHub           CommandReachability = "hub-or-route-mode"
	CommandReachContextAction CommandReachability = "contextual-action"
	CommandReachPaletteAlias  CommandReachability = "searchable-alias"
)

type commandFinalClassification struct {
	Reachability CommandReachability
	Target       string
}

// commandClassificationTable lists operations whose final reachability differs
// from the default contextual-action rule. Grouping aliases
// here makes those product decisions reviewable while every other registered
// operation remains preserved as a contextual action.
var commandClassificationTable = buildCommandClassificationTable()

func buildCommandClassificationTable() map[OperationID]commandFinalClassification {
	table := make(map[OperationID]commandFinalClassification)
	add := func(reachability CommandReachability, target string, names ...string) {
		for _, name := range names {
			table[operationIDForName(name)] = commandFinalClassification{Reachability: reachability, Target: target}
		}
	}

	add(CommandReachGlobalBinding, "?/Reference/Help", "help")
	add(CommandReachGlobalBinding, "Ctrl+C/Quit", "quit")
	add(CommandReachGlobalBinding, "Esc/Back", "back")
	add(CommandReachGlobalBinding, "Ctrl+N/TaskComposer", "quick capture")
	add(CommandReachGlobalBinding, "Ctrl+X/ProjectPicker", "switch project")
	add(CommandReachGlobalBinding, "|/RecentTasks", "recent tasks")
	add(CommandReachHub, "Home",
		"dashboard")
	add(CommandReachHub, "Planner",
		"today", "planner next", "planner backlog")
	add(CommandReachHub, "TaskWorkspace",
		"all tasks", "outliner")
	add(CommandReachHub, "ReviewHub",
		"daily review", "weekly review", "latest activity", "project recent activity", "metrics dashboard", "roadmap")
	add(CommandReachHub, "ManageHub",
		"inbox processor", "global inbox processor", "global notes", "global blockers", "global tag manager",
		"tag manager", "workflow rules", "scoring profiles", "task templates", "settings", "notifications", "message log")
	add(CommandReachViewMenu, "View menu",
		"all tasks by due date", "clear filters", "clear tag filter", "cycle note sort",
		"cycle note tag filter", "filter by context", "filter by section", "filter by tag", "outliner reset sort",
		"outliner sort by created", "outliner sort by due date", "outliner sort by priority", "outliner sort by score",
		"outliner sort by status", "section manager", "show notes", "view completed")
	add(CommandReachPaletteAlias, "Reference/Ask", "ask ai")
	add(CommandReachPaletteAlias, "ManageHub/Blockers", "blockers view")
	add(CommandReachPaletteAlias, "Palette/Search", "global search")
	add(CommandReachPaletteAlias, "TaskWorkspace/Outline/Project", "open project outliner")
	add(CommandReachPaletteAlias, "TaskWorkspace/Smart/HighPriority", "priority view")
	add(CommandReachPaletteAlias, "TaskWorkspace/Smart/ProjectFocus", "project focus queue")
	add(CommandReachPaletteAlias, "ReviewHub/Metrics/Project", "project metrics")
	add(CommandReachPaletteAlias, "Reference", "reference")
	return table
}

func classifyCommand(command Command) Command {
	if command.OperationID == "" {
		command.OperationID = operationIDForName(command.Name)
	}
	if command.Reachability != "" {
		if command.FinalTarget == "" {
			command.FinalTarget = "declared by command"
		}
		return command
	}
	if classification, ok := commandClassificationTable[command.OperationID]; ok {
		command.Reachability = classification.Reachability
		command.FinalTarget = classification.Target
		return command
	}
	command.Reachability = CommandReachContextAction
	command.FinalTarget = "active route or overlay contextual actions"
	return command
}

func operationIDForName(name string) OperationID {
	var slug strings.Builder
	separatorPending := false
	for _, r := range strings.TrimSpace(strings.ToLower(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separatorPending && slug.Len() > 0 {
				slug.WriteByte('.')
			}
			slug.WriteRune(r)
			separatorPending = false
			continue
		}
		separatorPending = true
	}
	if slug.Len() == 0 {
		slug.WriteString("unnamed")
	}
	return OperationID("tui.command." + slug.String())
}

// CommandInventoryEntry is one registered command variant. Variants that
// implement the same operation in different renderers share OperationID.
type CommandInventoryEntry struct {
	OperationID  OperationID
	Name         string
	Reachability CommandReachability
	FinalTarget  string
	Views        []PresentationID
}

// Inventory returns every registered command variant in stable operation/name/
// scope order, including variants whose dynamic Available predicate is false.
func (r *CommandRegistry) Inventory() []CommandInventoryEntry {
	inventory := make([]CommandInventoryEntry, 0)
	for _, variants := range r.commands {
		for _, command := range variants {
			inventory = append(inventory, CommandInventoryEntry{
				OperationID:  command.OperationID,
				Name:         command.Name,
				Reachability: command.Reachability,
				FinalTarget:  command.FinalTarget,
				Views:        append([]PresentationID(nil), command.Views...),
			})
		}
	}
	sort.Slice(inventory, func(i, j int) bool {
		if inventory[i].OperationID != inventory[j].OperationID {
			return inventory[i].OperationID < inventory[j].OperationID
		}
		if inventory[i].Name != inventory[j].Name {
			return inventory[i].Name < inventory[j].Name
		}
		return viewScopeKey(inventory[i].Views) < viewScopeKey(inventory[j].Views)
	})
	return inventory
}

func viewScopeKey(views []PresentationID) string {
	var key strings.Builder
	for _, view := range views {
		key.WriteString(view.String())
		key.WriteByte('|')
	}
	return key.String()
}
