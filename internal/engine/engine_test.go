package engine

import (
	"context"
	"dnsway-pc/internal/models"
	"testing"
)

func ctx() context.Context {
	return context.Background()
}

func TestDecide_P6_DefaultAllow(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "test", "example.com", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW, got %v", d.Decision)
	}
	if d.Reason != "P6: Default Allow" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P1_Allowlist(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "test", "khanacademy.org", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW for allowlisted domain, got %v", d.Decision)
	}
	if d.Reason != "P1: User Allowlist" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P1_AllowlistSubdomain(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "test", "www.khanacademy.org", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW for allowlisted subdomain, got %v", d.Decision)
	}
}

func TestDecide_P2_Denylist(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "test", "malicious-site.net", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for denylisted domain, got %v", d.Decision)
	}
	if d.Reason != "P2: User Denylist" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P2_DenylistSubdomain(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "test", "sub.malicious-site.net", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for denylisted subdomain, got %v", d.Decision)
	}
}

func TestDecide_P3_CategoryBlocked(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "test", "pornhub.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for porn category, got %v", d.Decision)
	}
	if d.Reason != "P3: Content Category Blocked" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P3_CategoryAllowedWhenNotBlocked(t *testing.T) {
	e := NewEngine()
	// CAT_004 (piracy) is not blocked and not in any time window
	d := e.Decide(ctx(), "test", "thepiratebay.org", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW for unblocked category, got %v", d.Decision)
	}
}

func TestDecide_P3_SubdomainCategoryBlocked(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "test", "www.pornhub.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for subdomain of blocked category, got %v", d.Decision)
	}
}

func TestDecide_P0_BypassPrevention(t *testing.T) {
	e := NewEngine()
	e.SetBypassPrevention("test", true)

	d := e.Decide(ctx(), "test", "nordvpn.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for bypass domain, got %v", d.Decision)
	}
	if d.Reason != "P0: Bypass Prevention (VPN/Proxy/Tor/DOH blocked)" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P0_BypassPreventionDisabled(t *testing.T) {
	e := NewEngine()
	e.SetBypassPrevention("test", false)

	d := e.Decide(ctx(), "test", "nordvpn.com", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW when bypass disabled, got %v", d.Decision)
	}
}

func TestDecide_P05_PhishingProtection(t *testing.T) {
	e := NewEngine()
	e.SetPhishingProtection("test", true)

	d := e.Decide(ctx(), "test", "paypa1.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for phishing domain, got %v", d.Decision)
	}
	if d.Reason != "P0.5: Phishing Protection" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P05_PhishingPunycode(t *testing.T) {
	e := NewEngine()
	e.SetPhishingProtection("test", true)

	d := e.Decide(ctx(), "test", "xn--paypl-8na.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for punycode phishing domain, got %v", d.Decision)
	}
}

func TestDecide_P06_CryptoJackingProtection(t *testing.T) {
	e := NewEngine()
	e.SetCryptoJackingProtection("test", true)

	d := e.Decide(ctx(), "test", "coinhive.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for crypto mining domain, got %v", d.Decision)
	}
	if d.Reason != "P0.6: Crypto Mining Protection" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P07_SuspectTLDBlocking(t *testing.T) {
	e := NewEngine()
	e.SetSuspectTLDBlocking("test", true)

	d := e.Decide(ctx(), "test", "suspicious-site.tk", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for suspect TLD, got %v", d.Decision)
	}
	if d.Reason != "P0.7: Suspect TLD Blocking" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P5_AIThreatDetection(t *testing.T) {
	e := NewEngine()
	e.SetAIThreatDetection("test", true)

	// High-entropy DGA-like domain
	d := e.Decide(ctx(), "test", "xqkzmnvbpqhrdtwlgjsf.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for AI threat domain, got %v", d.Decision)
	}
	if d.Reason != "P5: AI Threat Detection (DGA/Algorithmic Domain)" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P5_AIThreatLongAlphanumeric(t *testing.T) {
	e := NewEngine()
	e.SetAIThreatDetection("test", true)

	d := e.Decide(ctx(), "test", "abcdefghijklmnopqrstuvwxyz1234567890abcdef.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK for long alphanumeric DGA domain, got %v", d.Decision)
	}
}

func TestDecide_P55_SafeSearch(t *testing.T) {
	e := NewEngine()

	d := e.Decide(ctx(), "test", "google.com", "")
	if d.Decision != models.REWRITE {
		t.Errorf("expected REWRITE with SafeSearch rewrite, got %v", d.Decision)
	}
	if d.Domain != "forcesafesearch.google.com" {
		t.Errorf("expected rewritten domain, got %s", d.Domain)
	}
	if d.Reason != "P5.5: SafeSearch Rewrite (forcesafesearch.google.com)" {
		t.Errorf("unexpected reason: %s", d.Reason)
	}
}

func TestDecide_P55_SafeSearchDisabled(t *testing.T) {
	e := NewEngine()
	e.SetSafeSearch("test", false)

	d := e.Decide(ctx(), "test", "google.com", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW with SafeSearch disabled, got %v", d.Decision)
	}
	// Should not rewrite
	if d.Domain != "google.com" {
		t.Errorf("domain should not be rewritten when SafeSearch disabled, got %s", d.Domain)
	}
}

func TestDecide_PriorityOrder(t *testing.T) {
	// P0 (bypass) should take precedence over P1 (allowlist)
	e := NewEngine()
	e.SetBypassPrevention("test", true)
	e.AddToAllowlist("test", "nordvpn.com")

	d := e.Decide(ctx(), "test", "nordvpn.com", "")
	if d.Decision != models.Block {
		t.Errorf("P0 bypass should take precedence over allowlist, got %v", d.Decision)
	}
}

func TestUpdateUserConfig(t *testing.T) {
	e := NewEngine()
	e.UpdateUserConfig("test", "CAT_005", true)

	d := e.Decide(ctx(), "test", "steamcommunity.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK after enabling CAT_005, got %v", d.Decision)
	}
}

func TestAddRemoveAllowlist(t *testing.T) {
	e := NewEngine()

	d := e.Decide(ctx(), "test", "new-allowed-site.com", "")
	if d.Decision != models.Allow {
		t.Fatalf("expected ALLOW for new domain, got %v", d.Decision)
	}

	e.AddToAllowlist("test", "new-allowed-site.com")

	d = e.Decide(ctx(), "test", "new-allowed-site.com", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW after adding to allowlist, got %v", d.Decision)
	}
	if d.Reason != "P1: User Allowlist" {
		t.Errorf("expected P1 reason, got %s", d.Reason)
	}
}

func TestRemoveFromAllowlist(t *testing.T) {
	e := NewEngine()
	e.RemoveFromAllowlist("test", "khanacademy.org")

	// khanacademy.org is no longer allowlisted, so it should hit P6: allow
	d := e.Decide(ctx(), "test", "khanacademy.org", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW (default) after removing from allowlist, got %v", d.Decision)
	}
	if d.Reason == "P1: User Allowlist" {
		t.Error("remove from allowlist did not take effect")
	}
}

func TestAddRemoveDenylist(t *testing.T) {
	e := NewEngine()
	e.AddToDenylist("test", "new-blocked-site.com")

	d := e.Decide(ctx(), "test", "new-blocked-site.com", "")
	if d.Decision != models.Block {
		t.Errorf("expected BLOCK after adding to denylist, got %v", d.Decision)
	}

	e.RemoveFromDenylist("test", "new-blocked-site.com")
	d = e.Decide(ctx(), "test", "new-blocked-site.com", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW after removing from denylist, got %v", d.Decision)
	}
}

func TestSetYouTubeRestricted(t *testing.T) {
	e := NewEngine()
	e.SetYouTubeRestricted("test", true)
	// YouTube is CAT_006 which is NOT blocked by default in test profile
	// YouTube Restricted isn't a "decision" in the filter - it's a setting for restricted mode
	// It doesn't directly impact Decide; just test the setter works
	p := e.getProfile("test")
	if !p.YouTubeRestricted {
		t.Error("YouTubeRestricted should be true")
	}
}

func TestSetStorageRegion(t *testing.T) {
	e := NewEngine()
	e.SetStorageRegion("test", "eu")

	p := e.getProfile("test")
	if p.StorageRegion != "eu" {
		t.Errorf("expected eu, got %s", p.StorageRegion)
	}
}

func TestGetAllowlist(t *testing.T) {
	e := NewEngine()
	list := e.GetAllowlist("test")
	found := false
	for _, d := range list {
		if d == "khanacademy.org" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected khanacademy.org in allowlist")
	}
}

func TestListProfiles(t *testing.T) {
	e := NewEngine()
	profiles := e.ListProfiles()
	if len(profiles) == 0 {
		t.Fatal("expected at least one profile")
	}
	found := false
	for _, p := range profiles {
		if p == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected test profile in list")
	}
}

func TestNewProfileCreatedOnDemand(t *testing.T) {
	e := NewEngine()
	d := e.Decide(ctx(), "new-profile", "example.com", "")
	if d.Decision != models.Allow {
		t.Errorf("expected ALLOW for new profile, got %v", d.Decision)
	}
	profiles := e.ListProfiles()
	found := false
	for _, p := range profiles {
		if p == "new-profile" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new profile should have been created")
	}
}

func TestIsPhishingPunycodeHeuristic(t *testing.T) {
	e := NewEngine()
	e.SetPhishingProtection("test", true)

	tests := []struct {
		domain string
		want   bool
	}{
		{"xn--paypl-8na.com", true},
		{"xn--appl-qra.com", true},
		{"normal-site.com", false},
	}
	for _, tc := range tests {
		got := e.isPhishingDomain(tc.domain)
		if got != tc.want {
			t.Errorf("isPhishingDomain(%q) = %v, want %v", tc.domain, got, tc.want)
		}
	}
}

func TestIsPhishingBrandSubdomainHeuristic(t *testing.T) {
	e := NewEngine()
	e.SetPhishingProtection("test", true)

	// Brand name used as subdomain of a suspicious TLD
	got := e.isPhishingDomain("paypal.login.fake.tk")
	if !got {
		t.Error("expected phishing detection for brand subdomain on suspicious TLD")
	}
}

func TestIsSuspectTLD(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		domain string
		want   bool
	}{
		{"example.tk", true},
		{"example.ml", true},
		{"example.xyz", true},
		{"example.com", false},
		{"example.org", false},
	}
	for _, tc := range tests {
		got := e.isSuspectTLD(tc.domain)
		if got != tc.want {
			t.Errorf("isSuspectTLD(%q) = %v, want %v", tc.domain, got, tc.want)
		}
	}
}

func TestShannonEntropy(t *testing.T) {
	tests := []struct {
		s    string
		want float64
	}{
		{"aaaaaa", 0.0},
		{"abcdef", 2.58},
		{"xqkzmnvbpqhrdtwlgjsf", 4.22},  // High entropy DGA-like
	}
	for _, tc := range tests {
		got := shannonEntropy(tc.s)
		if got < tc.want-0.05 || got > tc.want+0.05 {
			t.Errorf("shannonEntropy(%q) = %.2f, want approximately %.2f", tc.s, got, tc.want)
		}
	}
}

func TestLookupCategories(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		domain string
		found  bool
		cat    string
	}{
		{"pornhub.com", true, "CAT_001"},
		{"www.pornhub.com", true, "CAT_001"},
		{"youtube.com", true, "CAT_006"},
		{"unknown-site.com", false, ""},
	}
	for _, tc := range tests {
		cats := e.lookupCategories(tc.domain)
		if tc.found {
			if len(cats) == 0 {
				t.Errorf("lookupCategories(%q): expected %s, got none", tc.domain, tc.cat)
			} else if cats[0] != tc.cat {
				t.Errorf("lookupCategories(%q): expected %s, got %v", tc.domain, tc.cat, cats)
			}
		} else {
			if len(cats) > 0 {
				t.Errorf("lookupCategories(%q): expected none, got %v", tc.domain, cats)
			}
		}
	}
}

func TestSetPersistenceCallback(t *testing.T) {
	e := NewEngine()
	called := false
	e.SetPersistenceCallback(func(profileID, domain string, decision int, reason string, categories []string, latencyUs int, clientIP string) error {
		called = true
		return nil
	})

	e.Decide(ctx(), "test", "example.com", "")
	if !called {
		t.Error("persistence callback should have been called")
	}
}

func TestGetDecisionHistoryAndStats(t *testing.T) {
	e := NewEngine()
	e.Decide(ctx(), "test", "example.com", "")
	e.Decide(ctx(), "test", "pornhub.com", "")

	history := e.GetDecisionHistory()
	if len(history) == 0 {
		t.Error("expected non-empty history")
	}

	stats := e.GetStats()
	if stats["pornhub.com"] != 1 {
		t.Errorf("expected 1 block for pornhub.com, got %d", stats["pornhub.com"])
	}
}
