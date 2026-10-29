package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRole(t *testing.T) {
	tests := []struct {
		input UserRole
		want  UserRole
	}{
		{RoleAdmin, RoleAdmin},
		{"Admin", RoleAdmin},
		{"ADMIN", RoleAdmin},
		{RoleUser, RoleUser},
		{"User", RoleUser},
		{"guest", RoleUser},
		{"", RoleUser},
	}

	for _, tt := range tests {
		got := NormalizeRole(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeRole(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateServerConfig(t *testing.T) {
	cfg := ServerConfig{
		Ports:           []int{2222, 99999, -5, 64235}, // 99999 and -5 are invalid
		MaxAuthAttempts: 0,
	}

	cfg.ValidateAndSetDefaults()

	// Verify ports were filtered to only valid ports
	if len(cfg.Ports) != 2 || cfg.Ports[0] != 2222 || cfg.Ports[1] != 64235 {
		t.Errorf("expected filtered ports [2222 64235], got %v", cfg.Ports)
	}

	// Verify defaults
	if cfg.MaxAuthAttempts != 3 {
		t.Errorf("expected default MaxAuthAttempts=3, got %d", cfg.MaxAuthAttempts)
	}
	if cfg.UDPIdleTimeoutSec != DefaultUDPIdleTimeoutSec {
		t.Errorf("expected default UDPIdleTimeoutSec=%d, got %d", DefaultUDPIdleTimeoutSec, cfg.UDPIdleTimeoutSec)
	}
	if cfg.UDPSocketBufferKB != 4096 {
		t.Errorf("expected default UDPSocketBufferKB=4096, got %d", cfg.UDPSocketBufferKB)
	}
	if cfg.Shell == "" {
		t.Error("expected non-empty default shell")
	}
}

func TestValidateUser(t *testing.T) {
	u := User{
		Username:          "test",
		Role:              "ADMIN",
		MaxSessions:       -1,
		SessionTTLSeconds: 0,
		MaxSpeedKBPS:      -100,
		MaxTotalMB:        -50,
	}

	u.ValidateAndSetDefaults()

	if u.Role != RoleAdmin {
		t.Errorf("expected normalized role %q, got %q", RoleAdmin, u.Role)
	}
	if u.MaxSessions != DefaultMaxSessionsPerUser {
		t.Errorf("expected default MaxSessions=%d, got %d", DefaultMaxSessionsPerUser, u.MaxSessions)
	}
	if u.SessionTTLSeconds != DefaultSessionTTLSeconds {
		t.Errorf("expected default SessionTTLSeconds=%d, got %d", DefaultSessionTTLSeconds, u.SessionTTLSeconds)
	}
	if u.MaxSpeedKBPS != 0 {
		t.Errorf("expected clamped MaxSpeedKBPS=0, got %d", u.MaxSpeedKBPS)
	}
	if u.MaxTotalMB != 0 {
		t.Errorf("expected clamped MaxTotalMB=0, got %d", u.MaxTotalMB)
	}
}

func TestLoadUsers_LegacyFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	legacyData := map[string]string{
		"user1": "pass1",
		"user2": "pass2",
	}
	raw, _ := json.Marshal(legacyData)

	usersPath := filepath.Join(tempDir, UsersFileName)
	if err := os.WriteFile(usersPath, raw, 0644); err != nil {
		t.Fatal(err)
	}

	// Verify legacy loader parsing
	users, err := ParseUsersData(raw)
	if err != nil {
		t.Fatalf("ParseUsersData failed: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users["user1"].Role != RoleUser {
		t.Errorf("expected role %q, got %q", RoleUser, users["user1"].Role)
	}
}
