package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"Abdal_4iProto_Server/core/config"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/ssh"
)

func TestCleanupExpiredAndRevokedSessions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_sessions.db")
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("failed to open bbolt: %v", err)
	}
	defer db.Close()

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
		t.Fatalf("failed to init buckets: %v", err)
	}

	sm := &Manager{
		db:                db,
		activeConnections: make(map[string]*ssh.ServerConn),
	}

	now := time.Now().Unix()

	// 1. Active session
	sActive := Info{
		SessionID: "s_active",
		Username:  "user1",
		LastSeen:  now,
		Revoked:   false,
	}
	// 2. Revoked session
	sRevoked := Info{
		SessionID: "s_revoked",
		Username:  "user1",
		LastSeen:  now,
		Revoked:   true,
	}
	// 3. Expired session
	sExpired := Info{
		SessionID: "s_expired",
		Username:  "user1",
		LastSeen:  now - 1000,
		Revoked:   false,
	}

	for _, s := range []Info{sActive, sRevoked, sExpired} {
		data, _ := json.Marshal(s)
		_ = db.Update(func(tx *bbolt.Tx) error {
			_ = tx.Bucket([]byte(config.BucketSessions)).Put([]byte(s.SessionID), data)
			uKey := s.Username + "|0|" + s.SessionID
			return tx.Bucket([]byte(config.BucketUserSessions)).Put([]byte(uKey), []byte(s.SessionID))
		})
	}

	sm.CleanupExpiredSessions()

	// Check results
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(config.BucketSessions))
		if b.Get([]byte("s_active")) == nil {
			t.Errorf("active session should not be removed")
		}
		if b.Get([]byte("s_revoked")) != nil {
			t.Errorf("revoked session should have been removed by cleanup")
		}
		if b.Get([]byte("s_expired")) != nil {
			t.Errorf("expired session should have been removed by cleanup")
		}
		return nil
	})
}
