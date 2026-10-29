package security

import (
	"testing"

	"Abdal_4iProto_Server/core/config"
)

func TestMatchesWildcard(t *testing.T) {
	tests := []struct {
		str     string
		pattern string
		want    bool
	}{
		// Exact matches
		{"example.com", "example.com", true},
		{"192.168.1.1", "192.168.1.1", true},
		{"example.com", "other.com", false},

		// Case insensitivity
		{"Example.COM", "example.com", true},
		{"mail.GOOGLE.com", "*.google.com", true},

		// Subdomain wildcard (*.domain.com)
		{"mail.google.com", "*.google.com", true},
		{"sub.mail.google.com", "*.google.com", true},
		{"google.com", "*.google.com", true},
		{"notgoogle.com", "*.google.com", false},
		{"fakegoogle.com", "*.google.com", false},

		// IP wildcard prefixes
		{"192.168.1.55", "192.168.1.*", true},
		{"192.168.2.55", "192.168.1.*", false},
		{"10.0.5.1", "10.*", true},

		// Substring wildcard (*part*)
		{"ads.tracker.net", "*tracker*", true},
		{"safe.site.org", "*tracker*", false},

		// Suffix wildcard
		{"site.ru", "*.ru", true},
		{"site.com", "*.ru", false},
	}

	for _, tt := range tests {
		got := matchesWildcard(tt.str, tt.pattern)
		if got != tt.want {
			t.Errorf("matchesWildcard(%q, %q) = %v; want %v", tt.str, tt.pattern, got, tt.want)
		}
	}
}

func TestIsDomainOrIPBlocked(t *testing.T) {
	config.SetUsers(map[string]config.User{
		"test_blocked_user": {
			Username:       "test_blocked_user",
			BlockedDomains: []string{"*.malware.com", "bad.org"},
			BlockedIPs:     []string{"192.168.100.*", "10.10.10.10"},
		},
	})

	// Non-existent user
	if IsDomainOrIPBlocked("non_existent", "bad.org") {
		t.Error("expected non-existent user to not be blocked")
	}

	// Blocked domains
	if !IsDomainOrIPBlocked("test_blocked_user", "bad.org") {
		t.Error("expected bad.org to be blocked")
	}
	if !IsDomainOrIPBlocked("test_blocked_user", "api.malware.com") {
		t.Error("expected api.malware.com to be blocked")
	}
	if !IsDomainOrIPBlocked("test_blocked_user", "malware.com") {
		t.Error("expected malware.com to be blocked by *.malware.com")
	}
	if IsDomainOrIPBlocked("test_blocked_user", "good.org") {
		t.Error("expected good.org to NOT be blocked")
	}

	// Blocked IPs
	if !IsDomainOrIPBlocked("test_blocked_user", "192.168.100.50") {
		t.Error("expected 192.168.100.50 to be blocked")
	}
	if !IsDomainOrIPBlocked("test_blocked_user", "10.10.10.10") {
		t.Error("expected 10.10.10.10 to be blocked")
	}
	if IsDomainOrIPBlocked("test_blocked_user", "192.168.200.50") {
		t.Error("expected 192.168.200.50 to NOT be blocked")
	}
}
