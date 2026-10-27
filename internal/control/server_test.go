package control

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// socketPath is short: a Unix socket path is capped near 104 bytes on macOS,
// which t.TempDir's nested names can exceed.
func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ttc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

func echoDispatch(req Request) <-chan Reply {
	replies := make(chan Reply, 1)
	replies <- Reply{OK: true, Message: req.Op + ":" + req.Task}
	return replies
}

type client struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

func dial(t *testing.T, path string) *client {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &client{conn: conn, scanner: bufio.NewScanner(conn)}
}

func (c *client) send(t *testing.T, line string) {
	t.Helper()
	if _, err := c.conn.Write([]byte(line + "\n")); err != nil {
		t.Fatal(err)
	}
}

func (c *client) next(t *testing.T) map[string]any {
	t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if !c.scanner.Scan() {
		t.Fatalf("no line: %v", c.scanner.Err())
	}
	var got map[string]any
	if err := json.Unmarshal(c.scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", c.scanner.Text(), err)
	}
	return got
}

func TestServerRepliesToEachRequestByID(t *testing.T) {
	path := socketPath(t)
	server, err := Listen(path, echoDispatch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("socket mode = %o, want 600", perm)
	}

	c := dial(t, path)
	c.send(t, `{"id":7,"op":"reveal","project":"p","task":"t1"}`)
	got := c.next(t)
	if got["reply"] != float64(7) || got["ok"] != true || got["message"] != "reveal:t1" {
		t.Fatalf("reply = %v", got)
	}

	c.send(t, `not json`)
	if got := c.next(t); got["ok"] != false {
		t.Fatalf("malformed request answered %v", got)
	}
}

func TestServerTimesOutASilentTUI(t *testing.T) {
	server, err := Listen(socketPath(t), func(Request) <-chan Reply { return make(chan Reply) })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	server.replyTimeout = 20 * time.Millisecond

	c := dial(t, server.Path())
	c.send(t, `{"id":1,"op":"capture","text":"x"}`)
	if got := c.next(t); got["ok"] != false || got["message"] != "Docket did not answer in time" {
		t.Fatalf("reply = %v", got)
	}
}

// TestServerPublishesEventsAndReplaysThePulse: a connection hears events
// published while it is open, and a late one hears the latest pulse and then
// the latest agenda first.
func TestServerPublishesEventsAndReplaysThePulse(t *testing.T) {
	server, err := Listen(socketPath(t), echoDispatch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	early := dial(t, server.Path())
	// The connection is registered once the server has answered on it.
	early.send(t, `{"id":1,"op":"reveal"}`)
	early.next(t)

	server.Publish(Event{Event: EventPulse, Pulse: &Pulse{Overdue: 3}})
	if got := early.next(t); got["event"] != "pulse" || got["pulse"].(map[string]any)["overdue"] != float64(3) {
		t.Fatalf("event = %v", got)
	}

	server.Publish(Event{Event: EventAgenda, Agenda: &Agenda{Total: 1, Items: []AgendaItem{{Kind: AgendaOverdue, Task: "t"}}}})
	if got := early.next(t); got["event"] != "agenda" {
		t.Fatalf("event = %v", got)
	}

	late := dial(t, server.Path())
	if got := late.next(t); got["event"] != "pulse" {
		t.Fatalf("a new connection heard %v first, want the pulse", got)
	}
	if got := late.next(t); got["event"] != "agenda" || got["agenda"].(map[string]any)["total"] != float64(1) {
		t.Fatalf("a new connection heard %v second, want the agenda", got)
	}
}

func TestServerCloseRemovesTheSocketAndReplacesAStaleOne(t *testing.T) {
	path := socketPath(t)
	first, err := Listen(path, echoDispatch)
	if err != nil {
		t.Fatal(err)
	}
	// A crashed session leaves its socket file; the next one replaces it.
	_ = first.listener.Close()
	if _, statErr := os.Stat(path); statErr != nil {
		t.Skip("this platform removed the socket with its listener")
	}
	second, err := Listen(path, echoDispatch)
	if err != nil {
		t.Fatalf("a stale socket blocked the next session: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket still present after Close: %v", err)
	}
}

func TestListenRefusesToReplaceAFile(t *testing.T) {
	path := socketPath(t)
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(path, echoDispatch); err == nil {
		t.Fatal("Listen replaced a regular file")
	}
}

// TestServerCloseEndsAQuittingHostsConnection: a quit's OK reply may not
// outlive the exit, so the host reads the connection ending as the same
// answer. Close must end every open connection rather than leave one hanging.
func TestServerCloseEndsAQuittingHostsConnection(t *testing.T) {
	server, err := Listen(socketPath(t), echoDispatch)
	if err != nil {
		t.Fatal(err)
	}
	c := dial(t, server.Path())
	c.send(t, `{"id":3,"op":"quit"}`)
	if got := c.next(t); got["reply"] != float64(3) || got["message"] != OpQuit+":" {
		t.Fatalf("reply = %v", got)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if c.scanner.Scan() {
		t.Fatalf("read %q after Close, want the connection ended", c.scanner.Text())
	}
	if err := c.scanner.Err(); err != nil {
		t.Fatalf("connection did not end cleanly: %v", err)
	}
}
