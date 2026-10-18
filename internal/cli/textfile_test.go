package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTextFileIsExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.txt")
	want := "She said \"yes\" -- line one\r\nline two & 'quotes' "
	if err := os.WriteFile(path, append([]byte("\xef\xbb\xbf"), want...), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readTextFile(path); err != nil || got != want {
		t.Fatalf("got %q, %v", got, err)
	}
	if err := os.WriteFile(path, []byte("  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTextFile(path); err == nil {
		t.Fatal("an empty file was accepted")
	}
}
