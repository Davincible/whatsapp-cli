// Package sqlitedb opens every SQLite database the CLI uses, through the pure-Go
// ncruces driver and its "adiantum" encrypting VFS, so the store can be built
// without cgo (and so for windows-amd64 from any machine) and is encrypted at
// rest.
//
// SPIKE: the key comes from a 0600 file beside the databases. The real build
// seals it with DPAPI on Windows and keeps it in the Keychain on macOS.
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

const keyFile = "store.key"

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

// StoreKey returns the 32-byte key for the store in dir, creating it on first use.
func StoreKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, keyFile)
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		k, derr := hex.DecodeString(string(b))
		if derr != nil || len(k) != 32 {
			return nil, fmt.Errorf("store key %s is corrupt", path)
		}
		return k, nil
	case errors.Is(err, os.ErrNotExist):
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(hex.EncodeToString(k)), 0o600); err != nil {
			return nil, err
		}
		return k, nil
	default:
		return nil, err
	}
}
