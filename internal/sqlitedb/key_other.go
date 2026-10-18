//go:build !windows

package sqlitedb

// keyFileName is a 0600 file holding the key in hex.
const keyFileName = "store.key"

func sealKey(b []byte) ([]byte, error)   { return b, nil }
func unsealKey(b []byte) ([]byte, error) { return b, nil }
