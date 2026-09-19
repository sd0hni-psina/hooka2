package realtime

import (
	"sync"
)

// sendBuffer is the per-connection queue, tech.md section 9.
const sendBuffer = 64

type Client struct {
	venueID   string
	staffID   string
	sessionID string

	send     chan []byte
	overflow chan struct{}
	closeMu  sync.Once

	mu          sync.Mutex
	hall        string
	subscribed  bool
	subscribing bool
	held        []held
}

type held struct {
	seq int64
	msg []byte
}

func newClient(venueID, staffID, sessionID string) *Client {
	return &Client{
		venueID: venueID, staffID: staffID, sessionID: sessionID,
		send:     make(chan []byte, sendBuffer),
		overflow: make(chan struct{}),
	}
}

func (c *Client) currentHall() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hall
}

// deliver queues a broadcast. Unsubscribed clients get nothing: without a
// snapshot base they cannot detect gaps.
func (c *Client) deliver(seq int64, msg []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case c.subscribing:
		if len(c.held) >= sendBuffer {
			c.markOverflow()
			return
		}
		c.held = append(c.held, held{seq, msg})
	case c.subscribed:
		c.push(msg)
	}
}

// push must run under c.mu.
func (c *Client) push(msg []byte) {
	select {
	case c.send <- msg:
	default:
		c.markOverflow()
	}
}

func (c *Client) markOverflow() {
	c.closeMu.Do(func() { close(c.overflow) })
}

func (c *Client) hold(hallID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hall = hallID
	c.subscribing = true
	c.held = c.held[:0]
}

// finishSubscribe sends the snapshot, then the broadcasts that arrived while
// it was read. Events already reflected in the snapshot may repeat; the
// client reducer applies them idempotently.
func (c *Client) finishSubscribe(snapshot []byte, base int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.push(snapshot)
	for _, h := range c.held {
		if h.seq > base {
			c.push(h.msg)
		}
	}
	c.held = nil
	c.subscribing = false
	c.subscribed = true
}

// abortSubscribe drops held events after a failed subscribe.
func (c *Client) abortSubscribe() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.held = nil
	c.subscribing = false
}

// reply queues a direct message.
func (c *Client) reply(msg []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.push(msg)
}
