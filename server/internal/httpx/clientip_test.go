package httpx

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveIP(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		xff    string
		trust  bool
		want   string
	}{
		{"direct client is its connection address", "203.0.113.7:5555", "", false, "203.0.113.7"},
		{"forwarded header is ignored without a trusted proxy", "203.0.113.7:5555", "1.2.3.4", false, "203.0.113.7"},
		{"trusted proxy gives the last forwarded address", "172.18.0.3:4444", "198.51.100.9", true, "198.51.100.9"},
		{"a spoofed first entry loses to the address the proxy appended", "172.18.0.3:4444", "1.2.3.4, 198.51.100.9", true, "198.51.100.9"},
		{"trusted proxy without the header falls back to the connection", "172.18.0.3:4444", "", true, "172.18.0.3"},
		{"trusted proxy with a garbage header falls back to the connection", "172.18.0.3:4444", "not-an-ip", true, "172.18.0.3"},
		{"blank trailing entry is skipped", "172.18.0.3:4444", "198.51.100.9, ", true, "198.51.100.9"},
		{"ipv4 mapped ipv6 becomes ipv4", "[::ffff:192.0.2.1]:80", "", false, "192.0.2.1"},
		{"ipv6 collapses to its 64 bit prefix", "[2001:db8:1:2:3:4:5:6]:80", "", false, "2001:db8:1:2::"},
		{"two hosts of one ipv6 prefix share an address", "[2001:db8:1:2:ffff:ffff:ffff:ffff]:80", "", false, "2001:db8:1:2::"},
		{"connection address without a port still parses", "203.0.113.7", "", false, "203.0.113.7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, netip.MustParseAddr(c.want), resolveIP(c.remote, c.xff, c.trust))
		})
	}

	t.Run("an unusable connection address gives no ip", func(t *testing.T) {
		require.False(t, resolveIP("@", "", false).IsValid())
	})
}

func TestClientIPMiddlewarePutsTheAddressInTheContext(t *testing.T) {
	var got netip.Addr
	h := ClientIP(true)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ClientIPFrom(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.18.0.3:4444"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	h.ServeHTTP(httptest.NewRecorder(), req)
	require.Equal(t, netip.MustParseAddr("198.51.100.9"), got)

	require.False(t, ClientIPFrom(req.Context()).IsValid(), "no middleware, no address")
}
