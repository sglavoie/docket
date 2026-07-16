package external

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

const defaultFzfTimeoutSeconds = 30

// fzfTimeoutSeconds is the timeout for fzf invocations. CLI workspace loads can
// overlap in tests and embedded callers, so process-global settings use atomic
// access just like the icon package's global presentation setting.
var fzfTimeoutSeconds atomic.Int64

func configuredFzfTimeoutSeconds() int {
	seconds := int(fzfTimeoutSeconds.Load())
	if seconds <= 0 {
		return defaultFzfTimeoutSeconds
	}
	return seconds
}

// SetFzfTimeout sets the fzf invocation timeout in seconds.
// Values <= 0 are ignored and the default (30s) is kept.
func SetFzfTimeout(seconds int) {
	if seconds > 0 {
		fzfTimeoutSeconds.Store(int64(seconds))
	}
}

// sanitizeFzfField replaces characters that would corrupt fzf's tab-delimited
// line format. Tabs become spaces and newlines/carriage-returns become spaces
// so a single field cannot span multiple lines or split columns.
func sanitizeFzfField(s string) string {
	r := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	return r.Replace(s)
}

// classifyFzfRunError maps a non-nil error from fzf's exec.Cmd.Run() to a
// stable error for callers. Returns ErrCanceled for fzf no-match/no-selection
// (exit code 1), fzf cancel/Ctrl-C (exit code 130), and parent-context
// cancellation, a descriptive timeout error for deadline expiry, and a wrapped
// error otherwise.
func classifyFzfRunError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return tterrors.ErrCanceled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("fzf timed out after %ds — try again or press Ctrl+C to cancel", configuredFzfTimeoutSeconds())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 1, 130:
			return tterrors.ErrCanceled
		}
	}
	return fmt.Errorf("fzf failed: %w", err)
}

// FzfAvailable checks if fzf is installed and available.
func FzfAvailable() bool {
	_, err := CachedFzfPath()
	return err == nil
}

// FzfSelectProject uses fzf to select a project from the list.
func FzfSelectProject(projects []*models.Project, prompt string) (*models.Project, error) {
	fzfPath, err := CachedFzfPath()
	if err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects to select from")
	}

	// Build input for fzf.
	// Format: slug<TAB>display-name (slug)<TAB>context<TAB>N tasks
	// The slug column is hidden via --with-nth=2.. and used only for parsing.
	var lines []string
	projectMap := make(map[string]*models.Project)

	for _, p := range projects {
		line := fmt.Sprintf("%s\t%s (%s)\t%s\t%d tasks",
			sanitizeFzfField(p.Slug),
			sanitizeFzfField(p.Name),
			sanitizeFzfField(p.Slug),
			sanitizeFzfField(p.Context),
			p.CountTasks())
		lines = append(lines, line)
		projectMap[p.Slug] = p
	}

	input := strings.Join(lines, "\n")

	// Run fzf
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configuredFzfTimeoutSeconds())*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, fzfPath,
		"--prompt", prompt+" ",
		"--height", "40%",
		"--reverse",
		"--delimiter", "\t",
		"--with-nth", "2..",
		"--preview-window", "hidden",
	)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, classifyFzfRunError(ctx, err)
	}

	selected := strings.TrimSpace(stdout.String())
	if selected == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse the selection - slug is the first tab-delimited field.
	parts := strings.SplitN(selected, "\t", 2)
	if len(parts) > 0 {
		if project, ok := projectMap[parts[0]]; ok {
			return project, nil
		}
	}

	return nil, fmt.Errorf("project not found")
}

// FzfSelectTask uses fzf to select a task from the list.
func FzfSelectTask(tasks []*models.Task, prompt string, previewCmd string) (*models.Task, error) {
	fzfPath, err := CachedFzfPath()
	if err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to select from")
	}

	// Build input for fzf
	var lines []string
	taskMap := make(map[int]*models.Task)

	for _, t := range tasks {
		line := fmt.Sprintf("#%d\t%s\t%s\t%s",
			t.ShortID,
			t.Type.Icon(),
			t.State().Icon(),
			sanitizeFzfField(t.Title))
		lines = append(lines, line)
		taskMap[t.ShortID] = t
	}

	input := strings.Join(lines, "\n")

	// Build fzf command with optional preview
	args := []string{
		"--prompt", prompt + " ",
		"--height", "50%",
		"--reverse",
		"--delimiter", "\t",
		"--with-nth", "1,2,3,4",
	}

	if previewCmd != "" {
		args = append(args,
			"--preview", previewCmd+" {1}",
			"--preview-window", "right:50%:wrap",
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configuredFzfTimeoutSeconds())*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, fzfPath, args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, classifyFzfRunError(ctx, err)
	}

	selected := strings.TrimSpace(stdout.String())
	if selected == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse the selection - get task ID from first field
	parts := strings.Split(selected, "\t")
	if len(parts) > 0 {
		// Extract short ID from "#N" format. Use strconv.Atoi for strict
		// parsing — fmt.Sscanf would accept trailing garbage (e.g. "12abc")
		// and silently match the wrong task.
		idStr := strings.TrimPrefix(parts[0], "#")
		if shortID, err := strconv.Atoi(idStr); err == nil {
			if task, ok := taskMap[shortID]; ok {
				return task, nil
			}
		}
	}

	return nil, fmt.Errorf("task not found")
}

// FzfMultiSelectProjects uses fzf to select multiple projects from the list.
func FzfMultiSelectProjects(projects []*models.Project, prompt string, preselectedIDs map[string]bool) ([]*models.Project, error) {
	fzfPath, err := CachedFzfPath()
	if err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects to select from")
	}

	// Build input for fzf.
	// Format: slug<TAB>display-name (slug)<TAB>context<TAB>N tasks
	// The slug column is hidden via --with-nth=2.. and used only for parsing.
	var lines []string
	projectMap := make(map[string]*models.Project)

	for _, p := range projects {
		marker := " "
		if preselectedIDs[p.ID] {
			marker = "*"
		}
		line := fmt.Sprintf("%s\t%s %s (%s)\t%s\t%d tasks",
			sanitizeFzfField(p.Slug),
			marker,
			sanitizeFzfField(p.Name),
			sanitizeFzfField(p.Slug),
			sanitizeFzfField(p.Context),
			p.CountTasks())
		lines = append(lines, line)
		projectMap[p.Slug] = p
	}

	input := strings.Join(lines, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configuredFzfTimeoutSeconds())*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, fzfPath,
		"--prompt", prompt+" ",
		"--height", "50%",
		"--reverse",
		"--multi",
		"--delimiter", "\t",
		"--with-nth", "2..",
	)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, classifyFzfRunError(ctx, err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse selections - slug is the first tab-delimited field.
	var selectedProjects []*models.Project
	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) > 0 && parts[0] != "" {
			if project, ok := projectMap[parts[0]]; ok {
				selectedProjects = append(selectedProjects, project)
			}
		}
	}

	if len(selectedProjects) == 0 {
		return nil, fmt.Errorf("no projects selected")
	}

	return selectedProjects, nil
}

// FzfMultiSelectTasks uses fzf to select multiple tasks from the list.
func FzfMultiSelectTasks(tasks []*models.Task, prompt string) ([]*models.Task, error) {
	fzfPath, err := CachedFzfPath()
	if err != nil {
		return nil, fmt.Errorf("fzf not found — install fzf (https://github.com/junegunn/fzf) or use the --id flag to specify items directly")
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks to select from")
	}

	// Build input for fzf
	var lines []string
	taskMap := make(map[int]*models.Task)

	for _, t := range tasks {
		line := fmt.Sprintf("#%d\t%s\t%s\t%s",
			t.ShortID,
			t.Type.Icon(),
			t.State().Icon(),
			sanitizeFzfField(t.Title))
		lines = append(lines, line)
		taskMap[t.ShortID] = t
	}

	input := strings.Join(lines, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configuredFzfTimeoutSeconds())*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, fzfPath,
		"--prompt", prompt+" ",
		"--height", "50%",
		"--reverse",
		"--multi",
		"--delimiter", "\t",
		"--with-nth", "1,2,3,4",
	)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, classifyFzfRunError(ctx, err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, tterrors.ErrCanceled
	}

	// Parse selections
	var selectedTasks []*models.Task
	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) > 0 {
			idStr := strings.TrimPrefix(parts[0], "#")
			if idStr == "" {
				continue
			}
			if shortID, err := strconv.Atoi(idStr); err == nil {
				if task, ok := taskMap[shortID]; ok {
					selectedTasks = append(selectedTasks, task)
				}
			}
		}
	}

	if len(selectedTasks) == 0 {
		return nil, fmt.Errorf("no tasks selected")
	}

	return selectedTasks, nil
}
