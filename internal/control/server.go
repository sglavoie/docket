package control

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sglavoie/docket/internal/logging"
)

// maxLineBytes bounds one request line. A capture may span several lines of
// text inside its JSON string, so the bound is generous, but a host that never
// writes a newline cannot grow Docket's memory without limit.
const maxLineBytes = 1 << 20

// outboxDepth is how many lines a connection may have queued before events
// for it are dropped. Replies are never dropped: a request waits for its own.
const outboxDepth = 64

// DefaultReplyTimeout is how long a request waits for the TUI to answer before
// the host is told it did not.
const DefaultReplyTimeout = 10 * time.Second

// Dispatch hands one request to the TUI and returns the channel its reply will
// arrive on. It must not block: the TUI answers from inside its own Update.
type Dispatch func(Request) <-chan Reply

// Server listens on one Unix socket for the life of a TUI session.
type Server struct {
	path         string
	listener     net.Listener
	dispatch     Dispatch
	replyTimeout time.Duration

	mu           sync.Mutex
	conns        map[*conn]struct{}
	lastPulse    []byte
	lastAgenda   []byte
	lastSchedule []byte
	lastEditing  []byte
	closed       bool
	wg           sync.WaitGroup
}

type conn struct {
	net.Conn
	out  chan []byte
	done chan struct{}
	once sync.Once
}

func (c *conn) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.Close()
	})
}

// Listen opens the socket at path, replacing a stale file a crashed session
// left there, and starts accepting. Only the user who runs Docket may connect.
func Listen(path string, dispatch Dispatch) (*Server, error) {
	if dispatch == nil {
		return nil, errors.New("control: a dispatch function is required")
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("control: %s exists and is not a socket", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("control: remove stale socket: %w", err)
		}
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("control: listen on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("control: restrict %s: %w", path, err)
	}
	s := &Server{
		path:         path,
		listener:     listener,
		dispatch:     dispatch,
		replyTimeout: DefaultReplyTimeout,
		conns:        map[*conn]struct{}{},
	}
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

// Path is the socket's path.
func (s *Server) Path() string { return s.path }

// Close stops accepting, ends every connection, and removes the socket.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	err := s.listener.Close()
	for _, c := range conns {
		c.close()
	}
	s.wg.Wait()
	if rmErr := os.Remove(s.path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) && err == nil {
		err = rmErr
	}
	return err
}

// Publish sends an event to every connection without blocking: a connection
// whose queue is full misses it. A pulse, an agenda, and a schedule are also
// remembered for the next connection.
func (s *Server) Publish(event Event) {
	line, err := json.Marshal(event)
	if err != nil {
		logging.Warn("control: encode %s event: %v", event.Event, err)
		return
	}
	line = append(line, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if event.Event == EventEditing {
		s.lastEditing = line
	}
	if event.Event == EventPulse {
		s.lastPulse = line
	}
	if event.Event == EventAgenda {
		s.lastAgenda = line
	}
	if event.Event == EventSchedule {
		s.lastSchedule = line
	}
	for c := range s.conns {
		select {
		case c.out <- line:
		default:
			logging.Warn("control: dropped a %s event for a slow host", event.Event)
		}
	}
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		raw, err := s.listener.Accept()
		if err != nil {
			return
		}
		c := &conn{Conn: raw, out: make(chan []byte, outboxDepth), done: make(chan struct{})}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			c.close()
			return
		}
		s.conns[c] = struct{}{}
		if s.lastEditing != nil {
			c.out <- s.lastEditing
		}
		if s.lastPulse != nil {
			c.out <- s.lastPulse
		}
		if s.lastAgenda != nil {
			c.out <- s.lastAgenda
		}
		if s.lastSchedule != nil {
			c.out <- s.lastSchedule
		}
		s.mu.Unlock()
		s.wg.Add(2)
		go s.write(c)
		go s.read(c)
	}
}

func (s *Server) drop(c *conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
	c.close()
}

func (s *Server) write(c *conn) {
	defer s.wg.Done()
	for {
		select {
		case <-c.done:
			return
		case line := <-c.out:
			if _, err := c.Write(line); err != nil {
				s.drop(c)
				return
			}
		}
	}
}

func (s *Server) read(c *conn) {
	defer s.wg.Done()
	defer s.drop(c)
	scanner := bufio.NewScanner(c)
	scanner.Buffer(make([]byte, 0, 4096), maxLineBytes)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			s.send(c, Reply{OK: false, Message: "malformed request: " + err.Error()})
			continue
		}
		s.send(c, s.answer(req, c.done))
	}
}

// answer waits for the TUI's reply to req, bounded by the reply timeout and by
// the connection ending.
func (s *Server) answer(req Request, done <-chan struct{}) Reply {
	replies := s.dispatch(req)
	timer := time.NewTimer(s.replyTimeout)
	defer timer.Stop()
	select {
	case reply := <-replies:
		reply.ID = req.ID
		return reply
	case <-timer.C:
		return Reply{ID: req.ID, OK: false, Message: "Docket did not answer in time"}
	case <-done:
		return Reply{ID: req.ID}
	}
}

// send queues a reply, waiting for room rather than dropping it.
func (s *Server) send(c *conn, reply Reply) {
	line, err := json.Marshal(reply)
	if err != nil {
		logging.Warn("control: encode reply: %v", err)
		return
	}
	select {
	case c.out <- append(line, '\n'):
	case <-c.done:
	}
}
