package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskform"
	"github.com/sglavoie/docket/internal/util"
)

// controlCapture reads text as the global `a` composer would and commits it as
// one capture transaction.
func (m *AppModel) controlCapture(text string, reply chan<- control.Reply, submissions ...storage.CaptureSubmission) tea.Cmd {
	preset, ok := m.captureComposerPreset(nil, nil)
	if !ok {
		answer(reply, false, "the Unfiled project is unavailable")
		return nil
	}
	reqs, lineErrors := taskform.ParseCaptureText(preset, m.domain.runtime, text)
	if len(lineErrors) > 0 {
		reply <- control.Reply{OK: false, Message: controlLineErrors(lineErrors, strings.Count(strings.TrimSpace(text), "\n") > 0), Selection: captureErrorSelection(text, lineErrors[0])}
		return nil
	}
	if len(reqs) == 0 {
		answer(reply, false, "Enter a task to capture")
		return nil
	}
	if len(submissions) > 0 {
		if err := submissions[0].Validate(); err != nil {
			answer(reply, false, err.Error())
			return nil
		}
		reqs[0].Submission = &submissions[0]
	}
	cmd := m.features.Capture.CaptureBulk(reqs, false)
	return func() tea.Msg {
		msg := cmd()
		captured, ok := msg.(coordinator.TasksCapturedMsg)
		if failure, failed := msg.(status.ErrorMsg); failed {
			var duplicate *storage.CaptureAlreadyCommitted
			if errors.As(failure.Err, &duplicate) {
				reply <- captureReceiptReply(duplicate.Receipt)
				return controlCommittedMsg{}
			}
		}
		if !ok || len(captured.Results) == 0 {
			answer(reply, false, commitRefusal(msg))
			return controlCommittedMsg{inner: msg}
		}
		summary := controlCapturedSummary(captured.Results)
		txID := captured.Results[0].TransactionID
		reply <- control.Reply{OK: true, Message: summary, Transaction: txID}
		return controlCommittedMsg{inner: captured, feedback: summary, txID: txID}
	}
}

// controlComplete answers what the global `a` composer would suggest for the
// draft up to the host's caret. No suggestion is still a success: it is the
// answer for a caret standing on prose.
func (m *AppModel) controlComplete(text string, reply chan<- control.Reply) {
	preset, ok := m.captureComposerPreset(nil, nil)
	if !ok {
		answer(reply, false, "the Unfiled project is unavailable")
		return
	}
	set := taskform.CaptureCompletions(preset, m.domain.runtime, text)
	result := control.Reply{OK: true}
	if set.Token != "" {
		completion := &control.Completion{Token: set.Token, Exact: set.Exact, Items: make([]control.CompletionItem, 0, len(set.Items))}
		for _, suggestion := range set.Items {
			completion.Items = append(completion.Items, control.CompletionItem{Label: suggestion.Label, Insert: suggestion.Insert})
		}
		result.Completion = completion
	}
	reply <- result
}

func (m *AppModel) controlPreview(text string, reply chan<- control.Reply) {
	preset, ok := m.captureComposerPreset(nil, nil)
	if !ok {
		answer(reply, false, "the Unfiled project is unavailable")
		return
	}
	preview, lineErrors := taskform.CapturePreviewText(preset, m.domain.runtime, text)
	if len(lineErrors) > 0 {
		answer(reply, false, controlLineErrors(lineErrors, strings.Contains(strings.TrimSpace(text), "\n")))
		return
	}
	reply <- control.Reply{OK: true, Preview: preview}
}

// adoptControlCommit reloads what a socket request committed through the save
// pipeline's own handler, then records the workspace a blurred session now
// holds, so the next focus does not mistake tt's own commit for another
// machine's.
func (m *AppModel) adoptControlCommit(msg controlCommittedMsg) tea.Cmd {
	_, refresh := m.handleGlobalSaveMessage(msg.inner)
	m.recordBlurredWorkspace()
	if msg.feedback == "" {
		return refresh
	}
	return tea.Batch(refresh, m.setRecoveryFeedback(msg.feedback, msg.txID, false))
}

func controlCapturedSummary(results []app.TaskResult) string {
	if len(results) == 1 {
		return "Captured: " + results[0].Title
	}
	return "Captured " + util.Count(len(results), "task")
}

func controlLineErrors(lineErrors []grammar.LineError, multiline bool) string {
	parts := make([]string, 0, len(lineErrors))
	for _, lineError := range lineErrors {
		if multiline {
			parts = append(parts, fmt.Sprintf("line %d: %s", lineError.Line, lineError.Message))
		} else {
			parts = append(parts, lineError.Message)
		}
	}
	return strings.Join(parts, "; ")
}

// Capture columns are runes; native text fields index UTF-16. Convert against
// the unchanged source and never search for a token that could appear twice.
func captureErrorSelection(text string, problem grammar.LineError) *control.TextSelection {
	lines := strings.Split(text, "\n")
	if problem.Line < 1 || problem.Line > len(lines) {
		return nil
	}
	line := []rune(lines[problem.Line-1])
	start := problem.Column
	if start < 0 || start >= len(line) || !strings.ContainsRune("!@#%$^*=>~&;", line[start]) {
		return nil
	}
	end := start + 1
	for end < len(line) && !unicode.IsSpace(line[end]) {
		end++
	}
	prefix := strings.Join(lines[:problem.Line-1], "\n")
	if problem.Line > 1 {
		prefix += "\n"
	}
	prefix += string(line[:start])
	return &control.TextSelection{Start: len(utf16.Encode([]rune(prefix))), Length: len(utf16.Encode(line[start:end]))}
}

type controlCaptureCheckedMsg struct {
	request control.Request
	reply   chan<- control.Reply
	receipt *storage.CaptureReceipt
	err     error
}

func (m *AppModel) lookupControlCapture(request control.Request, reply chan<- control.Reply) tea.Cmd {
	submission := storage.CaptureSubmission{ID: request.CaptureID, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(request.Text)))}
	if err := submission.Validate(); err != nil {
		answer(reply, false, err.Error())
		return nil
	}
	if m.domain.runtime == nil || m.domain.runtime.DataDir() == "" {
		answer(reply, false, "capture retry storage is unavailable")
		return nil
	}
	if request.Workspace == "" || request.Workspace != m.domain.runtime.Catalog().WorkspaceID {
		answer(reply, false, "This capture belongs to a different workspace. Reopen its workspace to retry it.")
		return nil
	}
	root := m.domain.runtime.DataDir()
	service := storage.NewCommitService(config.TransactionsDir(root), []string{root})
	return func() tea.Msg {
		receipt, err := service.LookupCaptureReceipt(submission)
		return controlCaptureCheckedMsg{request: request, reply: reply, receipt: receipt, err: err}
	}
}

func captureReceiptReply(receipt storage.CaptureReceipt) control.Reply {
	summary := "Captured " + util.Count(len(receipt.Titles), "task")
	if len(receipt.Titles) == 1 {
		summary = "Captured: " + receipt.Titles[0]
	}
	return control.Reply{OK: true, Message: summary, Transaction: receipt.TransactionID}
}
