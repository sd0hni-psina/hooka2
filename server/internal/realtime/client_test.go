package realtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func overflowed(c *Client) bool {
	select {
	case <-c.overflow:
		return true
	default:
		return false
	}
}

func TestClientQueue(t *testing.T) {
	t.Run("subscribed client overflows past 64 queued messages", func(t *testing.T) {
		c := newClient("v", "s", "sess")
		c.hold("h")
		c.finishSubscribe([]byte("snapshot"), 0)
		for i := 1; i < sendBuffer; i++ {
			c.deliver(int64(i), []byte("x"))
		}
		require.False(t, overflowed(c))
		c.deliver(64, []byte("x"))
		require.True(t, overflowed(c))
	})

	t.Run("held events older than the snapshot base are dropped", func(t *testing.T) {
		c := newClient("v", "s", "sess")
		c.hold("h")
		c.deliver(5, []byte("old"))
		c.deliver(6, []byte("new"))
		c.finishSubscribe([]byte("snapshot"), 5)
		require.Equal(t, "snapshot", string(<-c.send))
		require.Equal(t, "new", string(<-c.send))
		require.Empty(t, c.send)
	})
}
