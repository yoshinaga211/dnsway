package dns

import (
	"net"

	"github.com/miekg/dns"
)

// mockResponseWriter implements dns.ResponseWriter for testing.
type mockResponseWriter struct {
	msg    *dns.Msg
	remote net.Addr
	local  net.Addr
}

func (w *mockResponseWriter) LocalAddr() net.Addr         { return w.local }
func (w *mockResponseWriter) RemoteAddr() net.Addr        { return w.remote }
func (w *mockResponseWriter) WriteMsg(msg *dns.Msg) error { w.msg = msg; return nil }
func (w *mockResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *mockResponseWriter) Close() error                { return nil }
func (w *mockResponseWriter) TsigStatus() error           { return nil }
func (w *mockResponseWriter) TsigTimersOnly(b bool)       {}
func (w *mockResponseWriter) Hijack()                     {}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		remote: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345},
		local:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53},
	}
}
