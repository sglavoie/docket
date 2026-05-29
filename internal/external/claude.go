package external

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// ClaudeAvailable checks if the claude CLI is installed and available.
func ClaudeAvailable() bool {
	_, err := CachedClaudePath()
	return err == nil
}

// ClaudeAsk runs a query against the claude CLI and returns the full response.
func ClaudeAsk(ctx context.Context, query, systemPrompt string) (string, error) {
	claudePath, pathErr := CachedClaudePath()
	if pathErr != nil {
		return "", fmt.Errorf("claude not found in PATH: %w", pathErr)
	}

	args := []string{"--print", "-p", query}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, claudePath, args...)
	out, err := cmd.Output()
	if err != nil {
		// Prefer the context error when the parent canceled or the deadline
		// expired; the exec error would otherwise report a misleading
		// "signal: killed" / exit code -1 for what is really a cancel/timeout.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("claude exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run claude: %w", err)
	}
	return string(out), nil
}

// ClaudeAskStreaming starts a claude subprocess and returns its stdout for streaming.
func ClaudeAskStreaming(ctx context.Context, query, systemPrompt string) (io.ReadCloser, *exec.Cmd, error) {
	claudePath, pathErr := CachedClaudePath()
	if pathErr != nil {
		return nil, nil, fmt.Errorf("claude not found in PATH: %w", pathErr)
	}

	args := []string{"--print", "-p", query}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, claudePath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr into a buffer rather than inheriting the parent's
	// terminal stderr. The caller streams stdout into a TUI; if claude
	// writes to stderr (warnings, auth errors, deprecation notices) those
	// bytes would otherwise interleave with the bubbletea render and
	// corrupt the screen.
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start claude: %w", err)
	}

	return stdout, cmd, nil
}

// BuildTaskContext formats a project's tasks as a compact system prompt for the AI assistant.
func BuildTaskContext(project *models.Project) string {
	var b strings.Builder

	fmt.Fprintf(&b, "You are an AI assistant for the tt task manager. The current project is \"%s\".\n\n", project.Name)

	if len(project.Tasks) > 0 {
		b.WriteString("Current tasks:\n")
		for _, task := range project.Tasks {
			fmt.Fprintf(&b, "- #%d [%s] %s: \"%s\" (status: %s, priority: %d",
				task.ShortID, task.Type, task.Type.HeadingPrefix(), task.Title, task.Status, task.Priority)
			if task.Section != "" {
				fmt.Fprintf(&b, ", section: %s", task.Section)
			}
			if len(task.Tags) > 0 {
				fmt.Fprintf(&b, ", tags: %s", strings.Join(task.Tags, ","))
			}
			if task.DueDate != nil {
				dueFmt := "2006-01-02"
				if task.HasTime {
					dueFmt = "2006-01-02 15:04"
				}
				fmt.Fprintf(&b, ", due: %s", task.DueDate.Format(dueFmt))
			}
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(`Available tt commands (use these when the user wants to make changes):
  tt task add "Title" --type <epic|story|task|bug|idea> [--parent <id>] [--priority <1-5>] [--section <section>]
  tt add "Title !<priority> #<tag> @<section>"   (quick capture with inline syntax)
  tt task start <id>
  tt task complete <id>
  tt task cancel <id>
  tt task block <id>
  tt task priority <id> <1-5>
  tt task section <id> <section>
  tt task tag <id> <tag>
  tt task depends <task-id> <blocker-id>
  tt task snooze <id> <duration>
  tt task delete <id>

When suggesting changes, output the exact tt commands the user should run.
Wrap commands in a fenced code block.

Always respond using valid Markdown formatting.
`)

	return b.String()
}

// ExtractTTCommands parses a response for tt commands in fenced code blocks or standalone lines.
func ExtractTTCommands(response string) []string {
	var commands []string
	lines := strings.Split(response, "\n")
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			if strings.HasPrefix(trimmed, "tt ") {
				commands = append(commands, trimmed)
			}
		} else if strings.HasPrefix(trimmed, "tt ") && !strings.Contains(trimmed, " | ") {
			commands = append(commands, trimmed)
		}
	}

	return commands
}

// AIAssistant defines the interface for AI assistant integration.
type AIAssistant interface {
	Available() bool
	Ask(ctx context.Context, query, systemPrompt string) (string, error)
	AskStreaming(ctx context.Context, query, systemPrompt string) (io.ReadCloser, *exec.Cmd, error)
}

// ClaudeAssistant implements AIAssistant using the claude CLI.
type ClaudeAssistant struct{}

// NewClaudeAssistant creates a new claude-based assistant.
func NewClaudeAssistant() *ClaudeAssistant {
	return &ClaudeAssistant{}
}

// Available implements AIAssistant.
func (a *ClaudeAssistant) Available() bool {
	return ClaudeAvailable()
}

// Ask implements AIAssistant.
func (a *ClaudeAssistant) Ask(ctx context.Context, query, systemPrompt string) (string, error) {
	return ClaudeAsk(ctx, query, systemPrompt)
}

// AskStreaming implements AIAssistant.
func (a *ClaudeAssistant) AskStreaming(ctx context.Context, query, systemPrompt string) (io.ReadCloser, *exec.Cmd, error) {
	return ClaudeAskStreaming(ctx, query, systemPrompt)
}

// DefaultAssistant is the default AI assistant implementation.
var DefaultAssistant AIAssistant = NewClaudeAssistant()
