package sqlitedb

// SPIKE (throwaway): whatsmeow's own schema in an encrypted file, ciphertext on
// disk, and a wrong key refused.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.mau.fi/whatsmeow/store/sqlstore"
)

func TestWhatsmeowSchemaInEncryptedFile(t *testing.T) {
	dir := t.TempDir()
	key, err := StoreKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session.db")
	db, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	c := sqlstore.NewWithDB(db, "sqlite3", nil)
	if err := c.Upgrade(context.Background()); err != nil {
		t.Fatalf("whatsmeow upgrade: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE canary (v TEXT); INSERT INTO canary VALUES ('plaintext-canary-7f3a')`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name LIKE 'whatsmeow_%'`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("whatsmeow tables: %d, %v", n, err)
	}
	t.Logf("whatsmeow tables created: %d", n)
	_ = db.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("SQLite format 3")) || bytes.Contains(raw, []byte("plaintext-canary-7f3a")) || bytes.Contains(raw, []byte("whatsmeow_device")) {
		t.Fatal("database file holds plaintext")
	}
	t.Logf("file %d bytes, no SQLite header, no canary, no table names", len(raw))

	wrong := bytes.Repeat([]byte{7}, 32)
	db2, err := Open(path, wrong)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	err = db2.QueryRow(`SELECT count(*) FROM sqlite_master`).Scan(&n)
	if err == nil {
		t.Fatal("a wrong key read the database")
	}
	t.Logf("wrong key refused: %v", err)
}

func TestReopenWithRightKey(t *testing.T) {
	dir := t.TempDir()
	key, _ := StoreKey(dir)
	path := filepath.Join(dir, "m.db")
	for round := 0; round < 2; round++ {
		db, err := Open(path, key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS x (v TEXT); INSERT INTO x VALUES ('a')`); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM x`).Scan(&n); err != nil {
			t.Fatalf("round %d read: %v", round, err)
		}
		t.Logf("round %d rows %d", round, n)
		if err := db.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}
