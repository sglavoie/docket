package external

import (
	"github.com/sglavoie/tt/internal/models"
)

// FuzzySelector defines the interface for fuzzy-find selection of items.
// Implementations can use fzf, built-in TUI selectors, or mock implementations for testing.
type FuzzySelector interface {
	// Available returns true if the selector is available for use.
	Available() bool

	// SelectProject prompts the user to select a project from the list.
	SelectProject(projects []*models.Project, prompt string) (*models.Project, error)

	// SelectTask prompts the user to select a task from the list.
	// The previewCmd is an optional command to show task previews.
	SelectTask(tasks []*models.Task, prompt string, previewCmd string) (*models.Task, error)

	// MultiSelectTasks prompts the user to select multiple tasks.
	MultiSelectTasks(tasks []*models.Task, prompt string) ([]*models.Task, error)
}

// MarkdownRenderer defines the interface for rendering markdown content.
// Implementations can use glow, glamour, or mock implementations for testing.
type MarkdownRenderer interface {
	// Available returns true if the renderer is available for use.
	Available() bool

	// RenderMarkdown renders markdown content to stdout.
	RenderMarkdown(content string) error

	// RenderFile renders a markdown file to stdout.
	RenderFile(path string) error

	// RenderTaskMarkdown formats a task as markdown and renders it.
	RenderTaskMarkdown(task *models.Task) error

	// PageMarkdown renders markdown content with a pager.
	PageMarkdown(content string) error
}

// Editor defines the interface for external editor integration.
// Implementations can use VS Code, vim, or mock implementations for testing.
type Editor interface {
	// Available returns true if the editor is available for use.
	Available() bool

	// OpenFile opens a file in the editor.
	OpenFile(path string) error

	// OpenFileAtLine opens a file at a specific line number.
	OpenFileAtLine(path string, line int) error

	// OpenFolder opens a folder in the editor.
	OpenFolder(path string) error

	// OpenNewWindow opens a path in a new editor window.
	OpenNewWindow(path string) error

	// EditContent opens content in the editor and returns modified content.
	EditContent(content string, extension string) (string, error)
}

// FzfSelector implements FuzzySelector using fzf.
type FzfSelector struct{}

// NewFzfSelector creates a new fzf-based selector.
func NewFzfSelector() *FzfSelector {
	return &FzfSelector{}
}

// Available implements FuzzySelector.
func (s *FzfSelector) Available() bool {
	return FzfAvailable()
}

// SelectProject implements FuzzySelector.
func (s *FzfSelector) SelectProject(projects []*models.Project, prompt string) (*models.Project, error) {
	return FzfSelectProject(projects, prompt)
}

// SelectTask implements FuzzySelector.
func (s *FzfSelector) SelectTask(tasks []*models.Task, prompt string, previewCmd string) (*models.Task, error) {
	return FzfSelectTask(tasks, prompt, previewCmd)
}

// MultiSelectTasks implements FuzzySelector.
func (s *FzfSelector) MultiSelectTasks(tasks []*models.Task, prompt string) ([]*models.Task, error) {
	return FzfMultiSelectTasks(tasks, prompt)
}

// GlowRenderer implements MarkdownRenderer using glow.
type GlowRenderer struct{}

// NewGlowRenderer creates a new glow-based renderer.
func NewGlowRenderer() *GlowRenderer {
	return &GlowRenderer{}
}

// Available implements MarkdownRenderer.
func (r *GlowRenderer) Available() bool {
	return GlowAvailable()
}

// RenderMarkdown implements MarkdownRenderer.
func (r *GlowRenderer) RenderMarkdown(content string) error {
	return RenderMarkdown(content)
}

// RenderFile implements MarkdownRenderer.
func (r *GlowRenderer) RenderFile(path string) error {
	return RenderFile(path)
}

// RenderTaskMarkdown implements MarkdownRenderer.
func (r *GlowRenderer) RenderTaskMarkdown(task *models.Task) error {
	return RenderTaskMarkdown(task)
}

// PageMarkdown implements MarkdownRenderer.
func (r *GlowRenderer) PageMarkdown(content string) error {
	return PageMarkdown(content)
}

// VSCodeEditor implements Editor using VS Code.
type VSCodeEditor struct {
	editorOverride string // Optional override for editor command
}

// NewVSCodeEditor creates a new VS Code-based editor.
func NewVSCodeEditor() *VSCodeEditor {
	return &VSCodeEditor{}
}

// Available implements Editor.
func (e *VSCodeEditor) Available() bool {
	return VSCodeAvailable()
}

// OpenFile implements Editor.
func (e *VSCodeEditor) OpenFile(path string) error {
	return OpenFile(path)
}

// OpenFileAtLine implements Editor.
func (e *VSCodeEditor) OpenFileAtLine(path string, line int) error {
	return OpenFileAtLine(path, line)
}

// OpenFolder implements Editor.
func (e *VSCodeEditor) OpenFolder(path string) error {
	return OpenFolder(path)
}

// OpenNewWindow implements Editor.
func (e *VSCodeEditor) OpenNewWindow(path string) error {
	return OpenNewWindow(path)
}

// EditContent implements Editor.
func (e *VSCodeEditor) EditContent(content string, extension string) (string, error) {
	return EditInExternalEditor(e.editorOverride, content, extension)
}

// Default implementations for convenience.
var (
	DefaultSelector FuzzySelector    = NewFzfSelector()
	DefaultRenderer MarkdownRenderer = NewGlowRenderer()
	DefaultEditor   Editor           = NewVSCodeEditor()
)
