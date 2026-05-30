package session

import (
	"net"
	"testing"
)

// netConnWrapper mimics *crypto/tls.Conn / appleTLSConn which expose NetConn().
type netConnWrapper struct {
	net.Conn
	inner net.Conn
}

func (w netConnWrapper) NetConn() net.Conn { return w.inner }

// upstreamWrapper mimics bufio.CachedConn / badtls.ReadWaitConn which expose Upstream().
type upstreamWrapper struct {
	net.Conn
	inner net.Conn
}

func (w upstreamWrapper) Upstream() any { return w.inner }

// plainWrapper exposes neither hook — the chain must dead-end at nil, not panic.
type plainWrapper struct{ net.Conn }

func TestUnderlyingTCPConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	tcp, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	raw := tcp.(*net.TCPConn)

	// Mirrors production: CachedConn(Upstream) -> ReadWaitConn(Upstream) -> tls(NetConn) -> *net.TCPConn.
	wrapped := upstreamWrapper{Conn: raw, inner: upstreamWrapper{Conn: raw, inner: netConnWrapper{Conn: raw, inner: raw}}}

	if got := underlyingTCPConn(wrapped); got != raw {
		t.Fatalf("wrapped chain: got %v, want raw TCP conn", got)
	}
	if got := underlyingTCPConn(raw); got != raw {
		t.Fatalf("bare conn: got %v, want raw TCP conn", got)
	}
	if got := underlyingTCPConn(plainWrapper{Conn: raw}); got != nil {
		t.Fatalf("opaque wrapper: got %v, want nil", got)
	}

	// abortConnOnClose must reach the socket and SetLinger(0) without erroring out.
	abortConnOnClose(wrapped)
}
