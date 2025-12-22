package models

import (
	"regexp"
	"strings"
	"time"
)

// Project represents a collection of tasks.
type Project struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Context     string     `json:"context"`
	Path        string     `json:"path"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`

	// Computed fields (not persisted in config, stored in tasks.md)
	Tasks         []*Task           `json:"-"`
	NextShortID   int               `json:"-"`
	Sections      []string          `json:"-"` // Project-level section definitions
	SectionColors map[string]string `json:"-"` // Section name -> color name mapping
}

// NewProject creates a new project with default values.
func NewProject(id, name, path string) *Project {
	now := time.Now()
	return &Project{
		ID:            id,
		Slug:          Slugify(name),
		Name:          name,
		Context:       "personal",
		Path:          path,
		CreatedAt:     now,
		UpdatedAt:     now,
		Archived:      false,
		Tasks:         []*Task{},
		NextShortID:   1,
		Sections:      []string{SectionBacklog},
		SectionColors: make(map[string]string),
	}
}

// Slugify converts a name to a URL-safe slug.
func Slugify(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	reg := regexp.MustCompile("[^a-z0-9-]")
	slug = reg.ReplaceAllString(slug, "")
	reg = regexp.MustCompile("-+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

// Archive marks the project as archived.
func (p *Project) Archive() {
	now := time.Now()
	p.Archived = true
	p.ArchivedAt = &now
	p.UpdatedAt = now
}

// Unarchive removes the archived status.
func (p *Project) Unarchive() {
	p.Archived = false
	p.ArchivedAt = nil
	p.UpdatedAt = time.Now()
}

// Pin marks the project as pinned to appear at the top of lists.
func (p *Project) Pin() {
	now := time.Now()
	p.Pinned = true
	p.PinnedAt = &now
	p.UpdatedAt = now
}

// Unpin removes the pinned status.
func (p *Project) Unpin() {
	p.Pinned = false
	p.PinnedAt = nil
	p.UpdatedAt = time.Now()
}

// TogglePinned toggles the pinned status.
func (p *Project) TogglePinned() {
	if p.Pinned {
		p.Unpin()
	} else {
		p.Pin()
	}
}

// GetRootTasks returns tasks that have no parent.
func (p *Project) GetRootTasks() []*Task {
	var roots []*Task
	for _, task := range p.Tasks {
		if task.IsRoot() {
			roots = append(roots, task)
		}
	}
	return roots
}

// GetTaskByID finds a task by its UUID.
func (p *Project) GetTaskByID(id string) *Task {
	for _, task := range p.Tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

// GetTaskByShortID finds a task by its short numeric ID.
func (p *Project) GetTaskByShortID(shortID int) *Task {
	for _, task := range p.Tasks {
		if task.ShortID == shortID {
			return task
		}
	}
	return nil
}

// BuildTaskTree organizes flat tasks into a tree structure.
// Sets Parent, Children, and Level for each task.
func (p *Project) BuildTaskTree() {
	taskMap := make(map[string]*Task)
	for _, task := range p.Tasks {
		task.Children = []*Task{}
		task.Parent = nil
		task.Level = 0
		taskMap[task.ID] = task
	}

	for _, task := range p.Tasks {
		if task.ParentID != nil {
			if parent, ok := taskMap[*task.ParentID]; ok {
				task.Parent = parent
				parent.Children = append(parent.Children, task)
			}
		}
	}

	// Set levels via BFS
	var setLevels func(task *Task, level int)
	setLevels = func(task *Task, level int) {
		task.Level = level
		for _, child := range task.Children {
			setLevels(child, level+1)
		}
	}

	for _, task := range p.Tasks {
		if task.IsRoot() {
			setLevels(task, 0)
		}
	}
}

// GetNextShortID returns and increments the next available short ID.
func (p *Project) GetNextShortID() int {
	id := p.NextShortID
	p.NextShortID++
	return id
}

// CountTasks returns the total number of tasks.
func (p *Project) CountTasks() int {
	return len(p.Tasks)
}

// CountActiveTasks returns the number of non-completed tasks.
func (p *Project) CountActiveTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.Status.IsActive() {
			count++
		}
	}
	return count
}

// CountCompletedTasks returns the number of completed tasks.
func (p *Project) CountCompletedTasks() int {
	count := 0
	for _, task := range p.Tasks {
		if task.IsComplete() {
			count++
		}
	}
	return count
}

// GetTasksByStatus returns tasks filtered by status.
func (p *Project) GetTasksByStatus(status TaskStatus) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.Status == status {
			result = append(result, task)
		}
	}
	return result
}

// GetTasksByType returns tasks filtered by type.
func (p *Project) GetTasksByType(taskType TaskType) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.Type == taskType {
			result = append(result, task)
		}
	}
	return result
}

// GetTasksByTag returns tasks that have the given tag.
func (p *Project) GetTasksByTag(tag string) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		if task.HasTag(tag) {
			result = append(result, task)
		}
	}
	return result
}

// GetSiblings returns all tasks with the same parent as the given task,
// in their slice order. Includes the task itself.
func (p *Project) GetSiblings(task *Task) []*Task {
	var siblings []*Task
	for _, t := range p.Tasks {
		if (task.ParentID == nil && t.ParentID == nil) ||
			(task.ParentID != nil && t.ParentID != nil && *task.ParentID == *t.ParentID) {
			siblings = append(siblings, t)
		}
	}
	return siblings
}

// findTaskIndex returns the index of the task in the Tasks slice, or -1 if not found.
func (p *Project) findTaskIndex(task *Task) int {
	for i, t := range p.Tasks {
		if t.ID == task.ID {
			return i
		}
	}
	return -1
}

// GetTaskAbove returns the task that appears visually above the given task
// in a tree traversal (for indent operations). Returns nil if task is first.
func (p *Project) GetTaskAbove(task *Task) *Task {
	// Build the visual order by doing a depth-first traversal
	var visualOrder []*Task
	var traverse func(t *Task)
	traverse = func(t *Task) {
		visualOrder = append(visualOrder, t)
		for _, child := range t.Children {
			traverse(child)
		}
	}

	// First ensure tree is built
	p.BuildTaskTree()

	// Traverse all roots
	for _, root := range p.GetRootTasks() {
		traverse(root)
	}

	// Find the task and return the one above it
	for i, t := range visualOrder {
		if t.ID == task.ID {
			if i > 0 {
				return visualOrder[i-1]
			}
			return nil
		}
	}
	return nil
}

// MoveTaskUp swaps the task with its previous sibling in the Tasks slice.
// For root tasks, completed tasks cannot move above uncompleted tasks.
// Returns an error if the task cannot be moved up.
func (p *Project) MoveTaskUp(task *Task) error {
	siblings := p.GetSiblings(task)
	if len(siblings) < 2 {
		return nil // No siblings to swap with
	}

	// Find position among siblings
	var siblingIdx int = -1
	for i, s := range siblings {
		if s.ID == task.ID {
			siblingIdx = i
			break
		}
	}

	if siblingIdx <= 0 {
		return nil // Already at top
	}

	// Get the sibling above
	siblingAbove := siblings[siblingIdx-1]

	// For root tasks, enforce completion boundary:
	// A completed root task cannot move above an uncompleted root task
	if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
		return nil // Cannot cross completion boundary
	}

	// Find their positions in the main Tasks slice
	taskIdx := p.findTaskIndex(task)
	aboveIdx := p.findTaskIndex(siblingAbove)

	if taskIdx == -1 || aboveIdx == -1 {
		return nil
	}

	// Swap in the Tasks slice
	p.Tasks[taskIdx], p.Tasks[aboveIdx] = p.Tasks[aboveIdx], p.Tasks[taskIdx]
	task.UpdatedAt = time.Now()

	return nil
}

// MoveTaskDown swaps the task with its next sibling in the Tasks slice.
// For root tasks, uncompleted tasks cannot move below completed tasks.
// Returns an error if the task cannot be moved down.
func (p *Project) MoveTaskDown(task *Task) error {
	siblings := p.GetSiblings(task)
	if len(siblings) < 2 {
		return nil // No siblings to swap with
	}

	// Find position among siblings
	var siblingIdx int = -1
	for i, s := range siblings {
		if s.ID == task.ID {
			siblingIdx = i
			break
		}
	}

	if siblingIdx == -1 || siblingIdx >= len(siblings)-1 {
		return nil // Already at bottom
	}

	// Get the sibling below
	siblingBelow := siblings[siblingIdx+1]

	// For root tasks, enforce completion boundary:
	// An uncompleted root task cannot move below a completed root task
	if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
		return nil // Cannot cross completion boundary
	}

	// Find their positions in the main Tasks slice
	taskIdx := p.findTaskIndex(task)
	belowIdx := p.findTaskIndex(siblingBelow)

	if taskIdx == -1 || belowIdx == -1 {
		return nil
	}

	// Swap in the Tasks slice
	p.Tasks[taskIdx], p.Tasks[belowIdx] = p.Tasks[belowIdx], p.Tasks[taskIdx]
	task.UpdatedAt = time.Now()

	return nil
}

// IndentTask makes the task a child of the task visually above it.
// Returns an error if the operation is not valid.
func (p *Project) IndentTask(task *Task) error {
	// Get the task above in visual order
	targetParent := p.GetTaskAbove(task)
	if targetParent == nil {
		return nil // No task above
	}

	// Validate the indent operation
	if ok, _ := task.CanIndent(targetParent); !ok {
		return nil
	}

	// Update the parent reference
	task.ParentID = &targetParent.ID
	task.UpdatedAt = time.Now()

	// Rebuild the tree to update computed fields
	p.BuildTaskTree()

	return nil
}

// OutdentTask moves the task to be a sibling of its current parent.
// Returns an error if the operation is not valid.
func (p *Project) OutdentTask(task *Task) error {
	// Validate the outdent operation
	if ok, _ := task.CanOutdent(); !ok {
		return nil
	}

	// Get current parent before modifying
	currentParent := task.Parent
	if currentParent == nil {
		return nil // Already at root
	}

	// Set new parent to grandparent (or nil if parent is root)
	if currentParent.ParentID != nil {
		task.ParentID = currentParent.ParentID
	} else {
		task.ParentID = nil
	}
	task.UpdatedAt = time.Now()

	// Rebuild the tree to update computed fields
	p.BuildTaskTree()

	return nil
}

// GetBlockingTasks returns the tasks that the given task depends on.
func (p *Project) GetBlockingTasks(task *Task) []*Task {
	var blockers []*Task
	for _, depID := range task.DependsOn {
		if blocker := p.GetTaskByID(depID); blocker != nil {
			blockers = append(blockers, blocker)
		}
	}
	return blockers
}

// GetBlockedByTasks returns the tasks that depend on the given task.
func (p *Project) GetBlockedByTasks(task *Task) []*Task {
	var blocked []*Task
	for _, t := range p.Tasks {
		if t.HasDependency(task.ID) {
			blocked = append(blocked, t)
		}
	}
	return blocked
}

// HasCircularDependency checks if adding a dependency from taskID to blockerID
// would create a circular dependency.
func (p *Project) HasCircularDependency(taskID, blockerID string) bool {
	// If blocker depends (directly or transitively) on task, adding this dependency
	// would create a cycle.
	visited := make(map[string]bool)
	return p.dependsOnTransitive(blockerID, taskID, visited)
}

// dependsOnTransitive checks if source task depends on target task transitively.
func (p *Project) dependsOnTransitive(sourceID, targetID string, visited map[string]bool) bool {
	if sourceID == targetID {
		return true
	}
	if visited[sourceID] {
		return false
	}
	visited[sourceID] = true

	source := p.GetTaskByID(sourceID)
	if source == nil {
		return false
	}

	for _, depID := range source.DependsOn {
		if p.dependsOnTransitive(depID, targetID, visited) {
			return true
		}
	}
	return false
}

// CheckAndAutoUnblock checks all blocked tasks and transitions them to todo
// if all their dependencies are complete.
func (p *Project) CheckAndAutoUnblock() {
	for _, task := range p.Tasks {
		if task.Status != StatusBlocked || len(task.DependsOn) == 0 {
			continue
		}

		allComplete := true
		for _, depID := range task.DependsOn {
			dep := p.GetTaskByID(depID)
			if dep == nil || !dep.IsComplete() {
				allComplete = false
				break
			}
		}

		if allComplete {
			task.Status = StatusTodo
			task.UpdatedAt = time.Now()
		}
	}
}

// ReorderRootTasksByCompletion reorganizes the Tasks slice so that uncompleted
// root tasks appear before completed root tasks. Child tasks move with their
// parent root task. The relative order within each group is preserved.
func (p *Project) ReorderRootTasksByCompletion() {
	if len(p.Tasks) == 0 {
		return
	}

	// Build the tree to ensure parent-child relationships are set
	p.BuildTaskTree()

	// Collect root tasks with their subtrees (all descendants)
	type rootSubtree struct {
		root  *Task
		tasks []*Task // root + all descendants in original slice order
	}

	// Collect all descendants of a root task
	var collectDescendants func(task *Task, collected map[string]bool)
	collectDescendants = func(task *Task, collected map[string]bool) {
		collected[task.ID] = true
		for _, child := range task.Children {
			collectDescendants(child, collected)
		}
	}

	// Process each root task
	var uncompletedSubtrees []rootSubtree
	var completedSubtrees []rootSubtree
	processedTasks := make(map[string]bool)

	for _, task := range p.Tasks {
		if !task.IsRoot() || processedTasks[task.ID] {
			continue
		}

		// Collect all task IDs in this subtree
		subtreeIDs := make(map[string]bool)
		collectDescendants(task, subtreeIDs)

		// Gather tasks in original slice order
		var subtreeTasks []*Task
		for _, t := range p.Tasks {
			if subtreeIDs[t.ID] {
				subtreeTasks = append(subtreeTasks, t)
				processedTasks[t.ID] = true
			}
		}

		subtree := rootSubtree{root: task, tasks: subtreeTasks}
		if task.IsComplete() {
			completedSubtrees = append(completedSubtrees, subtree)
		} else {
			uncompletedSubtrees = append(uncompletedSubtrees, subtree)
		}
	}

	// Rebuild the Tasks slice: uncompleted subtrees first, then completed
	newTasks := make([]*Task, 0, len(p.Tasks))
	for _, subtree := range uncompletedSubtrees {
		newTasks = append(newTasks, subtree.tasks...)
	}
	for _, subtree := range completedSubtrees {
		newTasks = append(newTasks, subtree.tasks...)
	}

	p.Tasks = newTasks
}

// HasIncompleteDependencies returns true if the task has any dependencies
// that are not yet complete (done or cancelled).
func (p *Project) HasIncompleteDependencies(task *Task) bool {
	if len(task.DependsOn) == 0 {
		return false
	}
	for _, depID := range task.DependsOn {
		dep := p.GetTaskByID(depID)
		if dep != nil && !dep.IsComplete() {
			return true
		}
	}
	return false
}

// GetIncompleteDependencies returns the list of dependencies that are not
// yet complete. Useful for error messages when blocking task completion.
func (p *Project) GetIncompleteDependencies(task *Task) []*Task {
	var incomplete []*Task
	for _, depID := range task.DependsOn {
		dep := p.GetTaskByID(depID)
		if dep != nil && !dep.IsComplete() {
			incomplete = append(incomplete, dep)
		}
	}
	return incomplete
}

// GetSections returns the project's sections, ensuring Backlog is always first.
func (p *Project) GetSections() []string {
	if len(p.Sections) == 0 {
		return []string{SectionBacklog}
	}
	// Ensure Backlog is first
	hasBacklog := false
	for _, s := range p.Sections {
		if s == SectionBacklog {
			hasBacklog = true
			break
		}
	}
	if !hasBacklog {
		return append([]string{SectionBacklog}, p.Sections...)
	}
	// If Backlog exists but isn't first, reorder
	if p.Sections[0] != SectionBacklog {
		sections := make([]string, 0, len(p.Sections))
		sections = append(sections, SectionBacklog)
		for _, s := range p.Sections {
			if s != SectionBacklog {
				sections = append(sections, s)
			}
		}
		return sections
	}
	return p.Sections
}

// HasSection returns true if the section exists in the project.
func (p *Project) HasSection(name string) bool {
	for _, s := range p.Sections {
		if s == name {
			return true
		}
	}
	return false
}

// EnsureSection adds a section if it does not already exist.
func (p *Project) EnsureSection(name string) {
	if name == "" {
		return
	}
	if !p.HasSection(name) {
		p.Sections = append(p.Sections, name)
		p.UpdatedAt = time.Now()
	}
}

// RemoveSection removes a section from the project.
// Returns false if trying to remove the Backlog section or if section doesn't exist.
// Tasks in the removed section will have their section cleared.
func (p *Project) RemoveSection(name string) bool {
	if name == SectionBacklog {
		return false // Cannot remove Backlog
	}
	for i, s := range p.Sections {
		if s == name {
			p.Sections = append(p.Sections[:i], p.Sections[i+1:]...)
			// Clear section from tasks that had this section
			for _, task := range p.Tasks {
				if task.Section == name {
					task.Section = ""
					task.UpdatedAt = time.Now()
				}
			}
			// Remove color mapping
			if p.SectionColors != nil {
				delete(p.SectionColors, name)
			}
			p.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// RenameSection renames a section and updates all tasks with that section.
// Returns false if trying to rename Backlog or if the section doesn't exist.
func (p *Project) RenameSection(oldName, newName string) bool {
	if oldName == SectionBacklog || newName == "" {
		return false
	}
	if p.HasSection(newName) {
		return false // New name already exists
	}
	for i, s := range p.Sections {
		if s == oldName {
			p.Sections[i] = newName
			// Update tasks with this section
			for _, task := range p.Tasks {
				if task.Section == oldName {
					task.Section = newName
					task.UpdatedAt = time.Now()
				}
			}
			// Rename color mapping
			if p.SectionColors != nil {
				if color, exists := p.SectionColors[oldName]; exists {
					delete(p.SectionColors, oldName)
					p.SectionColors[newName] = color
				}
			}
			p.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// GetTasksBySection returns tasks filtered by section.
// When filtering by Backlog, tasks with empty section are also included.
func (p *Project) GetTasksBySection(section string) []*Task {
	var result []*Task
	for _, task := range p.Tasks {
		taskSection := task.Section
		if taskSection == "" {
			taskSection = SectionBacklog
		}
		if taskSection == section {
			result = append(result, task)
		}
	}
	return result
}

// CountTasksBySection returns the number of tasks in each section.
// Tasks with empty section are counted as Backlog.
func (p *Project) CountTasksBySection() map[string]int {
	counts := make(map[string]int)
	for _, task := range p.Tasks {
		section := task.Section
		if section == "" {
			section = SectionBacklog
		}
		counts[section]++
	}
	return counts
}

// GetSectionColor returns the color for a section, defaulting to "gray".
func (p *Project) GetSectionColor(section string) string {
	if p.SectionColors == nil {
		return "gray"
	}
	if color, ok := p.SectionColors[section]; ok && color != "" {
		return color
	}
	return "gray"
}

// SetSectionColor sets the color for a section.
func (p *Project) SetSectionColor(section, color string) {
	if p.SectionColors == nil {
		p.SectionColors = make(map[string]string)
	}
	p.SectionColors[section] = color
	p.UpdatedAt = time.Now()
}
