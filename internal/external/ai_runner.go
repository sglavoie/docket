package external

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sglavoie/tt/internal/actionplan"
	"github.com/sglavoie/tt/internal/aiharness"
)

const (
	// MaxHarnessOutputBytes is the most provider stdout retained per request.
	MaxHarnessOutputBytes = 1 << 20
	// MaxHarnessStderrBytes is the most provider stderr retained per request.
	MaxHarnessStderrBytes = 64 << 10
)

// ErrHarnessOutputLimit reports a response larger than MaxHarnessOutputBytes.
var ErrHarnessOutputLimit = errors.New("AI harness response exceeded the 1 MiB output limit")

// Request is one provider-neutral AI harness invocation. ConfigDir is an
// explicit one-request or persisted machine override; an empty value preserves
// the provider's inherited environment and native behavior.
type Request struct {
	Harness      aiharness.Name
	ConfigDir    string
	Query        string
	SystemPrompt string
}

// Runner starts blocking and streaming provider-neutral harness requests.
type Runner interface {
	Run(ctx context.Context, request Request) (string, error)
	Start(ctx context.Context, request Request) (Stream, error)
}

// Stream owns one harness process, including output reads, cancellation,
// waiting, bounded stderr capture, process reaping, and temporary cleanup.
// Wait is safe to call repeatedly or concurrently and always returns the same
// result.
type Stream interface {
	io.ReadCloser
	Cancel()
	Wait() error
	Stderr() string
	StderrTruncated() bool
}

type runnerDeps struct {
	lookup      func(aiharness.Name) (string, error)
	makeTempDir func() (string, error)
	removeAll   func(string) error
	environ     func() []string
}

type harnessRunner struct {
	deps runnerDeps
}

// NewHarnessRunner creates a runner backed by the supported CLIs on PATH.
func NewHarnessRunner() Runner {
	return newHarnessRunner(runnerDeps{
		lookup: CachedHarnessPath,
		makeTempDir: func() (string, error) {
			return os.MkdirTemp("", "tt-ask-")
		},
		removeAll: os.RemoveAll,
		environ:   os.Environ,
	})
}

func newHarnessRunner(deps runnerDeps) *harnessRunner {
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	return &harnessRunner{deps: deps}
}

// DefaultRunner is the injectable process boundary used by compatibility and
// user-facing AI flows.
var DefaultRunner Runner = NewHarnessRunner()

// HarnessAvailable reports whether the selected provider executable resolves.
func HarnessAvailable(harness aiharness.Name) bool {
	_, err := CachedHarnessPath(harness)
	return err == nil
}

func (r *harnessRunner) Run(ctx context.Context, request Request) (string, error) {
	stream, err := r.Start(ctx, request)
	if err != nil {
		return "", err
	}
	response, readErr := io.ReadAll(stream)
	waitErr := stream.Wait()
	if readErr != nil {
		return "", readErr
	}
	if waitErr != nil {
		return "", waitErr
	}
	return string(response), nil
}

func (r *harnessRunner) Start(ctx context.Context, request Request) (Stream, error) {
	if ctx == nil {
		return nil, fmt.Errorf("start AI harness: nil context")
	}
	info, err := aiharness.Info(request.Harness)
	if err != nil {
		return nil, err
	}
	executable, err := r.deps.lookup(request.Harness)
	if err != nil {
		return nil, fmt.Errorf("%s not found in PATH: %w", info.Executable, err)
	}

	configDir := ""
	if request.ConfigDir != "" {
		configDir, err = aiharness.NormalizeExplicitConfigDir(request.ConfigDir)
		if err != nil {
			return nil, err
		}
	}

	workDir, err := r.deps.makeTempDir()
	if err != nil {
		return nil, fmt.Errorf("create temporary AI harness directory: %w", err)
	}

	processCtx, cancel := context.WithCancel(ctx)
	stdout := newProcessOutput(MaxHarnessOutputBytes, cancel)
	stderr := newLimitedBuffer(MaxHarnessStderrBytes)
	cmd := exec.CommandContext(processCtx, executable, harnessArgs(request.Harness)...)
	cmd.Dir = workDir
	cmd.Env = r.deps.environ()
	if configDir != "" {
		cmd.Env = replaceEnvironmentValue(cmd.Env, info.ConfigEnvVar, configDir)
	}
	cmd.Stdin = strings.NewReader(providerPrompt(request.Query, request.SystemPrompt))
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Bound the case where a killed parent leaves a descendant holding inherited
	// stdout/stderr descriptors open.
	cmd.WaitDelay = time.Second

	if err := cmd.Start(); err != nil {
		cancel()
		cleanupErr := r.deps.removeAll(workDir)
		if ctxErr := ctx.Err(); ctxErr != nil {
			if cleanupErr != nil {
				return nil, fmt.Errorf("%w (also failed to remove temporary AI harness directory: %w)", ctxErr, cleanupErr)
			}
			return nil, ctxErr
		}
		if cleanupErr != nil {
			return nil, fmt.Errorf("start %s: %w (also failed to remove temporary AI harness directory: %w)", info.Executable, err, cleanupErr)
		}
		return nil, fmt.Errorf("start %s: %w", info.Executable, err)
	}

	process := &harnessProcess{
		harness:   request.Harness,
		parentCtx: ctx,
		ctx:       processCtx,
		cancel:    cancel,
		cmd:       cmd,
		stdout:    stdout,
		stderr:    stderr,
		workDir:   workDir,
		removeAll: r.deps.removeAll,
		done:      make(chan struct{}),
	}
	go process.reap()
	return process, nil
}

func harnessArgs(harness aiharness.Name) []string {
	switch harness {
	case aiharness.Claude:
		return []string{
			"--print",
			"--output-format", "text",
			"--permission-mode", "dontAsk",
			"--tools", "",
			"--no-session-persistence",
		}
	case aiharness.Codex:
		return []string{
			"exec",
			"--ephemeral",
			"--sandbox", "read-only",
			"--skip-git-repo-check",
			"--color", "never",
			"-",
		}
	case aiharness.Copilot:
		return []string{
			"--silent",
			"--no-ask-user",
			"--no-color",
			"--disable-builtin-mcps",
			"--deny-tool", "shell",
			"--deny-tool", "write",
		}
	default:
		return nil
	}
}

func replaceEnvironmentValue(environment []string, key, value string) []string {
	prefix := key + "="
	replaced := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			replaced = append(replaced, entry)
		}
	}
	return append(replaced, prefix+value)
}

type harnessProcess struct {
	harness   aiharness.Name
	parentCtx context.Context
	ctx       context.Context
	cancel    context.CancelFunc
	cmd       *exec.Cmd
	stdout    *processOutput
	stderr    *limitedBuffer
	workDir   string
	removeAll func(string) error
	done      chan struct{}

	resultMu   sync.Mutex
	result     error
	cleanupErr error
}

func (p *harnessProcess) Read(buffer []byte) (int, error) {
	return p.stdout.Read(buffer)
}

func (p *harnessProcess) Close() error {
	p.Cancel()
	result, cleanupErr := p.waitResult()
	if errors.Is(result, context.Canceled) && cleanupErr == nil {
		return nil
	}
	return result
}

func (p *harnessProcess) Cancel() {
	select {
	case <-p.done:
		return
	default:
		p.cancel()
	}
}

func (p *harnessProcess) Wait() error {
	result, _ := p.waitResult()
	return result
}

func (p *harnessProcess) waitResult() (error, error) {
	<-p.done
	p.resultMu.Lock()
	defer p.resultMu.Unlock()
	return p.result, p.cleanupErr
}

func (p *harnessProcess) Stderr() string {
	return actionplan.TerminalSafeDisplayText(p.stderr.String(), actionplan.DisplayTextMultiline)
}

func (p *harnessProcess) StderrTruncated() bool {
	return p.stderr.Truncated()
}

func (p *harnessProcess) reap() {
	commandErr := p.cmd.Wait()
	cleanupErr := p.removeAll(p.workDir)
	result := p.processResult(commandErr)
	if cleanupErr != nil {
		if result != nil {
			result = fmt.Errorf("%w (also failed to remove temporary AI harness directory: %w)", result, cleanupErr)
		} else {
			result = fmt.Errorf("remove temporary AI harness directory: %w", cleanupErr)
		}
	}

	p.resultMu.Lock()
	p.result = result
	p.cleanupErr = cleanupErr
	p.resultMu.Unlock()
	if p.stdout.Exceeded() {
		p.stdout.CloseWithError(ErrHarnessOutputLimit)
	} else {
		p.stdout.CloseWithError(nil)
	}
	p.cancel()
	close(p.done)
}

func (p *harnessProcess) processResult(commandErr error) error {
	if ctxErr := p.parentCtx.Err(); ctxErr != nil {
		return ctxErr
	}
	if p.stdout.Exceeded() {
		return ErrHarnessOutputLimit
	}
	if ctxErr := p.ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if commandErr != nil {
		var exitErr *exec.ExitError
		if errors.As(commandErr, &exitErr) {
			message := fmt.Sprintf("%s exited with code %d", p.harness, exitErr.ExitCode())
			if stderr := strings.TrimSpace(p.Stderr()); stderr != "" {
				message += ": " + stderr
			}
			if p.StderrTruncated() {
				message += " [stderr truncated at 64 KiB]"
			}
			return errors.New(message)
		}
		return fmt.Errorf("run %s: %w", p.harness, commandErr)
	}
	return nil
}

type processOutput struct {
	mu       sync.Mutex
	ready    *sync.Cond
	data     []byte
	total    int
	limit    int
	exceeded bool
	closed   bool
	closeErr error
	onLimit  func()
}

func newProcessOutput(limit int, onLimit func()) *processOutput {
	output := &processOutput{limit: limit, onLimit: onLimit}
	output.ready = sync.NewCond(&output.mu)
	return output
}

func (o *processOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	remaining := o.limit - o.total
	if remaining > len(data) {
		remaining = len(data)
	}
	if remaining > 0 {
		o.data = append(o.data, data[:remaining]...)
		o.total += remaining
	}
	newlyExceeded := len(data) > remaining && !o.exceeded
	if len(data) > remaining {
		o.exceeded = true
	}
	o.ready.Broadcast()
	o.mu.Unlock()

	if newlyExceeded && o.onLimit != nil {
		o.onLimit()
	}
	// Discard bytes beyond the cap without turning os/exec's copy goroutine
	// into the reported error; callers receive ErrHarnessOutputLimit instead.
	return len(data), nil
}

func (o *processOutput) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for len(o.data) == 0 && !o.closed {
		o.ready.Wait()
	}
	if len(o.data) > 0 {
		n := copy(buffer, o.data)
		o.data = o.data[n:]
		return n, nil
	}
	if o.closeErr != nil {
		return 0, o.closeErr
	}
	return 0, io.EOF
}

func (o *processOutput) CloseWithError(err error) {
	o.mu.Lock()
	o.closed = true
	o.closeErr = err
	o.ready.Broadcast()
	o.mu.Unlock()
}

func (o *processOutput) Exceeded() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.exceeded
}

type limitedBuffer struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - len(b.data)
	if remaining > len(data) {
		remaining = len(data)
	}
	if remaining > 0 {
		b.data = append(b.data, data[:remaining]...)
	}
	if remaining < len(data) {
		b.truncated = true
	}
	return len(data), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...))
}

func (b *limitedBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
