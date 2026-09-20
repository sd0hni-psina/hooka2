package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Broadcaster is the seam services publish through. v1 runs the hub in
// process; a Postgres LISTEN/NOTIFY fan-out can sit behind the same interface.
type Broadcaster interface {
	Publish(venueID string, hallID *string, typ string, data any)
}

// Hub keeps the connection registry and the per-venue seq.
//
// Every broadcast of a venue reaches every subscribed connection of that
// venue, whatever hall it watches, so seq stays gapless per connection. The
// client applies only events of its hall and tracks seq across all of them.
type Hub struct {
	log *slog.Logger
	now func() time.Time

	mu      sync.Mutex
	seq     map[string]int64
	clients map[string]map[*Client]struct{} // venue_id -> clients
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		log:     log,
		now:     time.Now,
		seq:     map[string]int64{},
		clients: map[string]map[*Client]struct{}{},
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.clients[c.venueID]
	if set == nil {
		set = map[*Client]struct{}{}
		h.clients[c.venueID] = set
	}
	set[c] = struct{}{}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.clients[c.venueID]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.venueID)
		}
	}
}

// Publish assigns the next seq and fans out under the hub lock, so two
// concurrent publishers cannot deliver seq out of order.
func (h *Hub) Publish(venueID string, hallID *string, typ string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq[venueID]++
	env := Envelope{V: ProtocolVersion, Type: typ, Seq: h.seq[venueID], TS: h.now().UTC(), VenueID: venueID, HallID: hallID, Data: data}
	msg, err := json.Marshal(env)
	if err != nil {
		h.log.Error("marshal ws event", "type", typ, "err", err)
		return
	}
	for c := range h.clients[venueID] {
		c.deliver(env.Seq, msg)
	}
}

// Seq returns the last seq issued for the venue.
func (h *Hub) Seq(venueID string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seq[venueID]
}

// beginSubscribe switches the client to a hall and starts holding broadcasts
// while the snapshot is read. The returned seq is the snapshot base.
func (h *Hub) beginSubscribe(c *Client, hallID string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.hold(hallID)
	return h.seq[c.venueID]
}

// direct builds a reply for one connection. It carries the current seq and
// does not advance it, tech.md section 19.1.
func (h *Hub) direct(venueID string, hallID *string, typ string, data any, seq int64) ([]byte, error) {
	msg, err := json.Marshal(Envelope{V: ProtocolVersion, Type: typ, Seq: seq, TS: h.now().UTC(), VenueID: venueID, HallID: hallID, Data: data})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", typ, err)
	}
	return msg, nil
}

// Online lists staff and sessions subscribed to a hall.
func (h *Hub) Online(venueID, hallID string) (staffIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	seen := map[string]bool{}
	for c := range h.clients[venueID] {
		if c.currentHall() == hallID && !seen[c.staffID] {
			seen[c.staffID] = true
			staffIDs = append(staffIDs, c.staffID)
		}
	}
	return staffIDs
}

// SessionOnline reports whether a device session holds an open connection.
func (h *Hub) SessionOnline(venueID, sessionID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[venueID] {
		if c.sessionID == sessionID {
			return true
		}
	}
	return false
}

// Snapshotter reads the HallState for state.snapshot.
type Snapshotter interface {
	HallState(ctx context.Context, venueID, hallID string, online []string, now time.Time) (any, error)
}

// CloseStaff drops every connection of a staff member, so a blocked master
// stops receiving events at once, tech.md section 19.11.
func (h *Hub) CloseStaff(venueID, staffID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[venueID] {
		if c.staffID == staffID {
			c.evict()
		}
	}
}
