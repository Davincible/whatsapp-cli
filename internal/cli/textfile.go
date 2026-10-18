package cli

import (
	"bytes"
	"fmt"
	"os"
)

// readTextFile returns a file's exact UTF-8 text (one leading byte-order mark
// dropped), so text a person approved is sent byte for byte. Windows
// PowerShell 5.1 mangles quotes inside a native program's arguments.
func readTextFile(path string) (string, error) {
	b, err := os.ReadFile(path) // #nosec G304 // the caller names its own file
	if err != nil {
		return "", fmt.Errorf("read --text-file: %w", err)
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(b)) == 0 {
		return "", fmt.Errorf("--text-file is empty")
	}
	return string(b), nil
}
