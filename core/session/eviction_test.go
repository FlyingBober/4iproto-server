package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"Abdal_4iProto_Server/core/config"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/ssh"
)

func setupTestDB(t *testing.T) (*bbolt.DB, func()) {
	tempDir, err := os.MkdirTemp("", "eviction_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test_sessions.db")
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open bbolt: %v", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(config.BucketSessions)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(config.BucketUserSessions)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to init buckets: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}
	return db, cleanup
}

func boolPtr(b bool) *bool {
	return &b
}

func TestCreateSession_EvictionDisabled_RejectsWhenLimitReached(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	config.SetServerConfig(config.ServerConfig{
		EvictOnMaxSessions: false,
	})
	config.SetUsers(map[string]config.User{
		"test_user": {
			Username:    "test_user",
			MaxSessions: 1,
		},
	})

	sm := &Manager{
		db:                db,
		activeConnections: make(map[string]*ssh.ServerConn),
	}

	s1, err := sm.CreateSession("test_user", "1.1.1.1", "SSH-2.0-Test")
	if err != nil {
		t.Fatalf("expected s1 to succeed, got: %v", err)
	}
	if s1 == "" {
		t.Fatal("expected non-empty s1")
	}

	s2, err := sm.CreateSession("test_user", "2.2.2.2", "SSH-2.0-Test")
	if err == nil {
		t.Fatalf("expected s2 to fail due to max sessions limit, but got session %s", s2)
	}
}

func TestCreateSession_EvictionEnabled_EvictsOldest(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	config.SetServerConfig(config.ServerConfig{
		EvictOnMaxSessions: true,
	})
	config.SetUsers(map[string]config.User{
		"test_user": {
			Username:    "test_user",
			MaxSessions: 1,
		},
	})

	sm := &Manager{
		db:                db,
		activeConnections: make(map[string]*ssh.ServerConn),
	}

	s1, err := sm.CreateSession("test_user", "1.1.1.1", "SSH-2.0-Test")
	if err != nil {
		t.Fatalf("expected s1 to succeed, got: %v", err)
	}

	s2, err := sm.CreateSession("test_user", "2.2.2.2", "SSH-2.0-Test")
	if err != nil {
		t.Fatalf("expected s2 to succeed via eviction, got: %v", err)
	}
	if s1 == s2 {
		t.Fatalf("expected different session IDs, got same: %s", s1)
	}

	// Verify s1 is revoked in database
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(config.BucketSessions))
		data := b.Get([]byte(s1))
		if data == nil {
			t.Fatalf("session s1 not found in bucket")
		}
		var s Info
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("unmarshal s1 failed: %v", err)
		}
		if !s.Revoked {
			t.Errorf("expected s1 to be marked Revoked: true, got false")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateSession_PerUserOverride(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Server default: EvictOnMaxSessions = false
	config.SetServerConfig(config.ServerConfig{
		EvictOnMaxSessions: false,
	})
	config.SetUsers(map[string]config.User{
		"user_evict_override": {
			Username:           "user_evict_override",
			MaxSessions:        1,
			EvictOnMaxSessions: boolPtr(true),
		},
		"user_no_override": {
			Username:    "user_no_override",
			MaxSessions: 1,
		},
	})

	sm := &Manager{
		db:                db,
		activeConnections: make(map[string]*ssh.ServerConn),
	}

	// 1. user_evict_override has user-level true -> should evict
	_, err := sm.CreateSession("user_evict_override", "1.1.1.1", "SSH-2.0-Test")
	if err != nil {
		t.Fatalf("failed to create s1: %v", err)
	}
	_, err = sm.CreateSession("user_evict_override", "2.2.2.2", "SSH-2.0-Test")
	if err != nil {
		t.Fatalf("expected s2 to succeed via user override eviction, got: %v", err)
	}

	// 2. user_no_override inherits server default (false) -> should reject
	_, err = sm.CreateSession("user_no_override", "1.1.1.1", "SSH-2.0-Test")
	if err != nil {
		t.Fatalf("failed to create s1 for user_no_override: %v", err)
	}
	_, err = sm.CreateSession("user_no_override", "2.2.2.2", "SSH-2.0-Test")
	if err == nil {
		t.Fatal("expected s2 for user_no_override to fail, but it succeeded")
	}

	// 3. User with explicit false when server default is true
	config.SetServerConfig(config.ServerConfig{
		EvictOnMaxSessions: true,
	})
	config.SetUsers(map[string]config.User{
		"user_disable_override": {
			Username:           "user_disable_override",
			MaxSessions:        1,
			EvictOnMaxSessions: boolPtr(false),
		},
	})
	_, err = sm.CreateSession("user_disable_override", "1.1.1.1", "SSH-2.0-Test")
	if err != nil {
		t.Fatalf("failed to create s1: %v", err)
	}
	_, err = sm.CreateSession("user_disable_override", "2.2.2.2", "SSH-2.0-Test")
	if err == nil {
		t.Fatal("expected s2 for user_disable_override to fail (override=false), but it succeeded")
	}
}
