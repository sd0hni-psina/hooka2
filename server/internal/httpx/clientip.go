package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type ipKey struct{}

// ClientIP stores the caller address in the request context, tech.md section
// 19.16. With trustProxy the last X-Forwarded-For entry wins: Caddy appends
// the address it saw, so entries the client sent earlier in the list never
// count. Without it the header is ignored, otherwise any client could pick
// its own address and walk around the per ip limits.
func ClientIP(trustProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), trustProxy)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ipKey{}, ip)))
		})
	}
}

// ClientIPFrom returns the address ClientIP stored, or an invalid Addr.
func ClientIPFrom(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(ipKey{}).(netip.Addr)
	return ip
}

func resolveIP(remoteAddr, forwarded string, trustProxy bool) netip.Addr {
	if trustProxy {
		parts := strings.Split(forwarded, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			entry := strings.TrimSpace(parts[i])
			if entry == "" {
				continue
			}
			if ip, err := netip.ParseAddr(entry); err == nil {
				return normalizeIP(ip)
			}
			break
		}
	}
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return normalizeIP(ip)
}

// normalizeIP maps IPv4-in-IPv6 to IPv4 and cuts IPv6 to its /64 prefix: one
// subscriber holds a whole /64, so per host addresses would be free to rotate.
func normalizeIP(ip netip.Addr) netip.Addr {
	ip = ip.Unmap().WithZone("")
	if ip.Is6() {
		return netip.PrefixFrom(ip, 64).Masked().Addr()
	}
	return ip
}
