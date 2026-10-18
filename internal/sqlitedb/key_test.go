package sqlitedb

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStoreKeyIsStableAcrossOpens(t *testing.T) {
	dir := t.TempDir()
	a, err := StoreKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := StoreKey(dir)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatalf("second open: %x, %v; want %x", b, err, a)
	}
}

// Processes starting together must agree on one key, or one of them encrypts
// a database the other can never read.
func TestStoreKeyConcurrentCreationSharesOneKey(t *testing.T) {
	dir := t.TempDir()
	const n = 16
	keys := make([][]byte, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, err := StoreKey(dir)
			if err != nil {
				t.Error(err)
			}
			keys[i] = k
		}()
	}
	wg.Wait()
	for i := 1; i < n; i++ {
		if !bytes.Equal(keys[0], keys[i]) {
			t.Fatalf("opener %d got a different key", i)
		}
	}
}

// A store whose key file is gone cannot be opened, and must not be given a new
// key: that would leave it unreadable and look like success.
func TestStoreKeyRefusesAMissingKeyBesideAStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "session.db"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StoreKey(dir); err == nil {
		t.Fatal("a new key was made for an existing store")
	}
	if _, err := os.Stat(filepath.Join(dir, keyFileName)); !os.IsNotExist(err) {
		t.Fatalf("a key file was written: %v", err)
	}
}
