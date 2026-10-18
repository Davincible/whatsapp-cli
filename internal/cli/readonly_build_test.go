//go:build readonly

package cli

import (
	"io"
	"testing"
)

// The tree the readonly binary ships is exactly the allowlist.
func TestReadOnlyBuildTree(t *testing.T) {
	got := map[string]bool{}
	for _, p := range commandPaths(rootCmd) {
		got[p] = true
	}
	for _, p := range readOnlyCommandPaths() {
		if !got[p] {
			t.Errorf("allowed command %q is missing", p)
		}
		delete(got, p)
	}
	for p := range got {
		if p != "help" && p != "completion" && !hasPrefix(p, "completion ") {
			t.Errorf("command %q is in the readonly tree", p)
		}
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

// Every way of changing WhatsApp fails as an unknown command, before any store
// or network is touched.
func TestReadOnlyBuildRefusesEverySend(t *testing.T) {
	for _, args := range [][]string{
		{"send", "123@s.whatsapp.net", "hello"},
		{"send", "123@s.whatsapp.net", "--text-file", "msg.txt"},
		{"react", "ABC", "x"},
		{"forward", "123@s.whatsapp.net", "ABC"},
		{"delete", "ABC"},
		{"groups", "join", "code"},
		{"groups", "leave", "123@g.us"},
		{"groups", "rename", "123@g.us", "x"},
		{"auth", "logout"},
	} {
		rootCmd.SetArgs(args)
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		err := rootCmd.Execute()
		if err == nil {
			t.Errorf("%v ran in a readonly build", args)
			continue
		}
		if !contains(err.Error(), "unknown command") && !contains(err.Error(), "accepts at most") {
			t.Errorf("%v: %v", args, err)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
