// Package sqlitedb opens every SQLite database the CLI uses, through the pure-Go
// ncruces driver and its "adiantum" encrypting VFS, so the store can be built
// without cgo (and so for windows-amd64 from any machine) and is encrypted at
// rest. The key sits beside the databases, sealed with DPAPI for the current
// user on Windows (key_windows.go) and as a 0600 file elsewhere (key_other.go).
package sqlitedb

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum"
)

// Open opens (creating if needed) the encrypted database at path with key.
// The key is applied by PRAGMA on every new connection, never put in the URI,
// so it cannot leak through the filename or an error message.
//
// FTS5 is an extension in this driver, not compiled in, and it is registered
// here, after the key, never with sqlite3.AutoExtension: an auto-extension runs
// before the key is set, reads the encrypted file, and every reopen then fails
// with "unable to open database file" (measured in the spike, 12 Oct 2026).
func Open(path string, key []byte) (*sql.DB, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("store key must be 32 bytes, got %d", len(key))
	}
	hexkey := hex.EncodeToString(key)
	return driver.Open(uri(path), func(c *sqlite3.Conn) error {
		for _, stmt := range []string{
			"PRAGMA hexkey='" + hexkey + "'",
			"PRAGMA foreign_keys=ON",
			"PRAGMA busy_timeout=10000",
			"PRAGMA temp_store=MEMORY",
		} {
			if err := c.Exec(stmt); err != nil {
				return err
			}
		}
		return fts5.Register(c)
	})
}

// uri builds a file: URI for path with the adiantum VFS. On Windows the drive
// letter path becomes file:///C:/..., which SQLite accepts.
func uri(path string) string {
	p := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p, RawQuery: "vfs=adiantum"}
	return u.String()
}

// StoreKey returns the 32-byte key for the store in dir, creating it on first
// use. Creation is exclusive, so two processes starting at once share one key.
// A missing key beside an existing database is an error, never a new key: a
// new key would make the store unreadable.
func StoreKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, keyFileName)
	b, err := os.ReadFile(path) // #nosec G304 // the store's own key file
	if err == nil {
		return decodeKey(path, b)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, db := range []string{"session.db", "messages.db"} {
		if _, err := os.Stat(filepath.Join(dir, db)); err == nil {
			return nil, fmt.Errorf("the store in %s has no key file (%s), so it cannot be opened; log in again to start a new store", dir, keyFileName)
		}
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	sealed, err := sealKey([]byte(hex.EncodeToString(k)))
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 // the store's own key file
	if errors.Is(err, os.ErrExist) {
		b, err := os.ReadFile(path) // #nosec G304 // another process made it first
		if err != nil {
			return nil, err
		}
		return decodeKey(path, b)
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(sealed); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return k, nil
}

func decodeKey(path string, raw []byte) ([]byte, error) {
	plain, err := unsealKey(raw)
	if err != nil {
		return nil, err
	}
	k, err := hex.DecodeString(string(plain))
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("store key %s is corrupt", path)
	}
	return k, nil
}
