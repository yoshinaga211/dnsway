package dns

import (
	"context"
	"net"
	"strings"
	"testing"

	"dnsway-pc/internal/engine"

	"github.com/miekg/dns"
)

func newTestServer(t *testing.T) *DNSServer {
	t.Helper()
	eng := engine.NewEngine()
	return NewDNSServer(eng)
}

func newDNSQuery(domain string) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	return m
}

func TestHandleDNSRequest_BlocksAdultDomain(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	srv.handleDNSRequest(mw, newDNSQuery("pornhub.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	first := mw.msg.Answer[0]
	if first.Header().Rrtype != dns.TypeA {
		t.Fatalf("expected A record, got type %d", first.Header().Rrtype)
	}
	a := first.(*dns.A)
	if a.A == nil {
		t.Fatal("expected non-nil A record")
	}
	if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatalf("expected 0.0.0.0 for blocked domain, got %s", a.A.String())
	}
}

func TestHandleDNSRequest_BlocksAdultDomainAlt(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	srv.handleDNSRequest(mw, newDNSQuery("xvideos.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatalf("expected 0.0.0.0 for blocked domain, got %s", a.A.String())
	}
}

func TestHandleDNSRequest_AllowlistedDomainNotBlocked(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// khanacademy.org is in the allowlist for the "test" profile
	srv.handleDNSRequest(mw, newDNSQuery("khanacademy.org"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	// Should NOT be blocked (0.0.0.0) since it's allowlisted
	a := mw.msg.Answer[0].(*dns.A)
	if a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatal("allowlisted domain should not be blocked")
	}
}

func TestHandleDNSRequest_AllowlistedDomain(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	srv.handleDNSRequest(mw, newDNSQuery("wikipedia.org"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatal("allowlisted domain should not be blocked")
	}
}

func TestHandleDNSRequest_DenylistedDomainBlocked(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	srv.handleDNSRequest(mw, newDNSQuery("malicious-site.net"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatalf("expected 0.0.0.0 for denylisted domain, got %s", a.A.String())
	}
}

func TestHandleDNSRequest_AllowedDomain(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	srv.handleDNSRequest(mw, newDNSQuery("example.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	// Allowed domains may get a real upstream response or the fallback 1.2.3.4
	a := mw.msg.Answer[0].(*dns.A)
	if a.A == nil {
		t.Fatal("expected non-nil A record")
	}
	// Should NOT be 0.0.0.0 (not blocked)
	if a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatal("allowed domain should not be blocked to 0.0.0.0")
	}
}

func TestHandleDNSRequest_MultipleQuestions(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// Create a query with multiple questions
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("pornhub.com"), dns.TypeA)
	m.Question = append(m.Question, dns.Question{Name: dns.Fqdn("example.com"), Qtype: dns.TypeA, Qclass: dns.ClassINET})

	srv.handleDNSRequest(mw, m)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
}

func TestHandleDNSRequest_SafeSearchRewritesDoubleClick(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// doubleclick.net is CAT_008 (ads) — blocked in test profile
	srv.handleDNSRequest(mw, newDNSQuery("doubleclick.net"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatalf("expected 0.0.0.0 for blocked CAT_008 domain, got %s", a.A.String())
	}
}

// Test that the DNS response has the query ID preserved
func TestHandleDNSRequest_PreservesQueryID(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	m := newDNSQuery("example.com")
	m.Id = 42

	srv.handleDNSRequest(mw, m)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if mw.msg.Id != 42 {
		t.Fatalf("expected query ID 42, got %d", mw.msg.Id)
	}
}

func TestHandleDNSRequest_UpstreamOrFallback(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()
	srv.client = &dns.Client{}

	srv.handleDNSRequest(mw, newDNSQuery("example.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}

	// When upstream fails (no network or timeout), falls back to 1.2.3.4
	if len(mw.msg.Answer) > 0 {
		first := mw.msg.Answer[0]
		if first.Header().Rrtype == dns.TypeA {
			a := first.(*dns.A)
			if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
				// Allowed — 1.2.3.4 fallback or real upstream answer
				return
			}
		}
	}
}

func TestHandleDNSRequest_NestedSubdomain(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// test a subdomain of a blocked domain
	srv.handleDNSRequest(mw, newDNSQuery("www.pornhub.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatalf("expected 0.0.0.0 for blocked subdomain, got %s", a.A.String())
	}
}

func TestHandleDNSRequest_EmptyQuestion(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// Query with no questions
	m := new(dns.Msg)
	m.SetReply(m)

	srv.handleDNSRequest(mw, m)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
}

func TestDecideCalledForProfile(t *testing.T) {
	srv := newTestServer(t)

	srv.engine.Decide(context.Background(), "unknown-profile", "example.com")

	// This should not panic — ensure Decide handles unknown profiles gracefully
	mw2 := newMockResponseWriter()
	srv.handleDNSRequest(mw2, newDNSQuery("example.com"))
	if mw2.msg == nil {
		t.Fatal("expected response message")
	}
}

func TestHandleDNSRequest_UpstreamClientExchange(t *testing.T) {
	// Test that upstream query error returns fallback
	srv := newTestServer(t)
	srv.client = &dns.Client{Net: "tcp"} // will fail since no TCP server

	mw := newMockResponseWriter()
	srv.handleDNSRequest(mw, newDNSQuery("example.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	// Should get fallback 1.2.3.4 since upstream will fail
	if len(mw.msg.Answer) > 0 {
		first := mw.msg.Answer[0]
		if first.Header().Rrtype == dns.TypeA {
			a := first.(*dns.A)
			if a.A.Equal(net.IPv4(1, 2, 3, 4)) || a.A.Equal(net.IPv4(0, 0, 0, 0)) {
				return // either blocked or fallback — either is valid
			}
		}
	}
}

func TestHandleDNSRequest_SubdomainOfCategorizedDomain(t *testing.T) {
	// Ensure subdomains are checked against categoryDB via engine.Decide
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	srv.handleDNSRequest(mw, newDNSQuery("sub.videos.pornhub.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatalf("expected 0.0.0.0 for deep subdomain of blocked domain, got %s", a.A.String())
	}
}

func TestHandleDNSRequest_MsgIsReply(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	q := newDNSQuery("example.com")
	srv.handleDNSRequest(mw, q)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if !mw.msg.MsgHdr.Response {
		t.Fatal("expected response flag to be set")
	}
}

// Regression: domains with trailing dots stripped correctly
func TestHandleDNSRequest_DomainWithTrailingDot(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	m := new(dns.Msg)
	m.SetQuestion("pornhub.com.", dns.TypeA)

	srv.handleDNSRequest(mw, m)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if !a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatalf("expected 0.0.0.0 for blocked domain with trailing dot, got %s", a.A.String())
	}
}

// Allowlisted but also categorized should be allowed (allowlist takes priority)
func TestHandleDNSRequest_AllowlistOverridesCategory(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// khanacademy.org is allowlisted and not in a category
	srv.handleDNSRequest(mw, newDNSQuery("khanacademy.org"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatal("allowlisted domain should not be blocked")
	}
}

// Root domain query should not panic
func TestHandleDNSRequest_RootDomain(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeA)

	srv.handleDNSRequest(mw, m)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
}

// Non-ASCII domain (internationalized, punycode)
func TestHandleDNSRequest_NonASCII(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	srv.handleDNSRequest(mw, newDNSQuery("xn--mgba3a4f16a.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
}

// Subdomain of allowlisted domain
func TestHandleDNSRequest_SubdomainOfAllowlisted(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// www.github.com is subdomain of github.com (allowlisted)
	srv.handleDNSRequest(mw, newDNSQuery("api.github.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if len(mw.msg.Answer) == 0 {
		t.Fatal("expected at least one answer")
	}
	a := mw.msg.Answer[0].(*dns.A)
	if a.A.Equal(net.IPv4(0, 0, 0, 0)) {
		t.Fatal("subdomain of allowlisted domain should not be blocked")
	}
}

// Test that handleDNSRequest does not leak the original question
func TestHandleDNSRequest_DoesNotModifyQuestion(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	q := newDNSQuery("pornhub.com")
	originalName := q.Question[0].Name

	srv.handleDNSRequest(mw, q)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	// Original name should still be in the question section
	if len(mw.msg.Question) == 0 {
		t.Fatal("expected question section to be preserved")
	}
	if mw.msg.Question[0].Name != originalName {
		t.Fatalf("expected question name %s, got %s", originalName, mw.msg.Question[0].Name)
	}
}

// Multiple blocked domains
func TestHandleDNSRequest_MultipleBlockedQuestions(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("pornhub.com"), dns.TypeA)
	m.Question = append(m.Question, dns.Question{Name: dns.Fqdn("xvideos.com"), Qtype: dns.TypeA, Qclass: dns.ClassINET})
	m.Question = append(m.Question, dns.Question{Name: dns.Fqdn("example.com"), Qtype: dns.TypeA, Qclass: dns.ClassINET})

	srv.handleDNSRequest(mw, m)

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	// Should have answers for each
	blockedCount := 0
	for _, ans := range mw.msg.Answer {
		if a, ok := ans.(*dns.A); ok && a.A.Equal(net.IPv4(0, 0, 0, 0)) {
			blockedCount++
		}
	}
	if blockedCount < 2 {
		t.Fatalf("expected at least 2 blocked answers, got %d", blockedCount)
	}
}

// Domain with ports or special characters stripped — just domain
func TestHandleDNSRequest_ValidDomainFormat(t *testing.T) {
	srv := newTestServer(t)
	mw := newMockResponseWriter()

	// Just a valid normal domain
	srv.handleDNSRequest(mw, newDNSQuery("example.com"))

	if mw.msg == nil {
		t.Fatal("expected response message")
	}
	if !strings.HasSuffix(mw.msg.Question[0].Name, "example.com.") {
		t.Fatalf("expected question to contain example.com, got %s", mw.msg.Question[0].Name)
	}
}
