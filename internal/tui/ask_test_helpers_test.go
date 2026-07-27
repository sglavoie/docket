package tui

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
)

type fakeAskRunner struct {
	mu       sync.Mutex
	streams  []external.Stream
	startErr error
	requests []external.Request
}

func (runner *fakeAskRunner) Run(ctx context.Context, request external.Request) (string, error) {
	stream, err := runner.Start(ctx, request)
	if err != nil {
		return "", err
	}
	data, readErr := io.ReadAll(stream)
	waitErr := stream.Wait()
	if readErr != nil {
		return "", readErr
	}
	return string(data), waitErr
}

func (runner *fakeAskRunner) Start(_ context.Context, request external.Request) (external.Stream, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	runner.requests = append(runner.requests, request)
	if runner.startErr != nil {
		return nil, runner.startErr
	}
	if len(runner.streams) == 0 {
		return nil, errors.New("no fake stream configured")
	}
	stream := runner.streams[0]
	runner.streams = runner.streams[1:]
	return stream, nil
}

func (runner *fakeAskRunner) Requests() []external.Request {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return append([]external.Request(nil), runner.requests...)
}

type scriptedAskStream struct {
	mu          sync.Mutex
	data        []byte
	offset      int
	chunkSize   int
	waitErr     error
	canceled    bool
	cancelOnce  sync.Once
	cancelCh    chan struct{}
	blockRead   bool
	readStarted chan struct{}
	readOnce    sync.Once
}

func newScriptedAskStream(data []byte, chunkSize int, waitErr error) *scriptedAskStream {
	return &scriptedAskStream{
		data: append([]byte(nil), data...), chunkSize: chunkSize,
		waitErr: waitErr, cancelCh: make(chan struct{}),
	}
}

func newBlockingAskStream() *scriptedAskStream {
	return &scriptedAskStream{
		chunkSize: 1, cancelCh: make(chan struct{}),
		blockRead: true, readStarted: make(chan struct{}),
	}
}

func (stream *scriptedAskStream) Read(buffer []byte) (int, error) {
	stream.mu.Lock()
	block, canceled := stream.blockRead, stream.canceled
	stream.mu.Unlock()
	if canceled {
		return 0, context.Canceled
	}
	if block {
		stream.readOnce.Do(func() { close(stream.readStarted) })
		<-stream.cancelCh
		return 0, context.Canceled
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.offset >= len(stream.data) {
		return 0, io.EOF
	}
	size := stream.chunkSize
	if size <= 0 || size > len(buffer) {
		size = len(buffer)
	}
	n := min(size, len(stream.data)-stream.offset)
	copy(buffer, stream.data[stream.offset:stream.offset+n])
	stream.offset += n
	return n, nil
}

func (stream *scriptedAskStream) Close() error {
	stream.Cancel()
	return stream.Wait()
}

func (stream *scriptedAskStream) Cancel() {
	stream.mu.Lock()
	stream.canceled = true
	stream.mu.Unlock()
	stream.cancelOnce.Do(func() { close(stream.cancelCh) })
}

func (stream *scriptedAskStream) Wait() error {
	stream.mu.Lock()
	block := stream.blockRead
	stream.mu.Unlock()
	if block {
		<-stream.cancelCh
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.canceled {
		return context.Canceled
	}
	return stream.waitErr
}

func (stream *scriptedAskStream) Stderr() string        { return "" }
func (stream *scriptedAskStream) StderrTruncated() bool { return false }

func (stream *scriptedAskStream) Canceled() bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return stream.canceled
}

func askStartMessage(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil submit command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatal("submit command did not return a batch")
	}
	return batch[0]()
}

func driveAskCommand(t *testing.T, model *AskViewModel, cmd tea.Cmd) {
	t.Helper()
	msg := askStartMessage(t, cmd)
	for step := 0; step < 10000; step++ {
		_, next := model.Update(msg)
		if next == nil {
			return
		}
		msg = next()
	}
	t.Fatal("ask command did not settle")
}

func driveAskPreferenceSave(t *testing.T, model *AskViewModel, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil preference-save command")
	}
	msg := cmd()
	saved, ok := msg.(AskPreferencesSavedMsg)
	if !ok || saved.Err != nil || saved.OperationID == 0 || saved.OperationID != model.preferenceSaveID {
		t.Fatalf("preference-save message = %#v", msg)
	}
	_, next := model.Update(msg)
	return next
}
