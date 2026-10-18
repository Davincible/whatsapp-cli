package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// readOnlyCommandPaths is every command a `readonly` build keeps: a positive
// allowlist, so a command added later is absent from a readonly build until
// someone adds it here on purpose. A command belongs here only if it can never
// send, change or delete anything on WhatsApp.
//
// Deliberately absent: send, react, forward, delete, groups join/leave/rename
// (they act on WhatsApp), auth logout (unlinks the device), contacts and export
// (bulk reads of an address book or a whole chat), alias (writes local state
// nobody here needs).
func readOnlyCommandPaths() []string {
	return []string{
		"auth", "auth login", "auth status",
		"chats", "messages", "search", "context", "groups",
		"download", "sync", "backfill", "watch", "doctor",
	}
}

// pruneToReadOnly removes every command not in readOnlyCommandPaths. Cobra's
// help and completion commands are added lazily at Execute and are unaffected.
func pruneToReadOnly(root *cobra.Command) {
	keep := make(map[string]struct{})
	for _, p := range readOnlyCommandPaths() {
		keep[p] = struct{}{}
	}
	var prune func(c *cobra.Command, prefix string)
	prune = func(c *cobra.Command, prefix string) {
		for _, sub := range c.Commands() {
			path := strings.TrimSpace(prefix + " " + sub.Name())
			if _, ok := keep[path]; !ok {
				c.RemoveCommand(sub)
				continue
			}
			prune(sub, path)
		}
	}
	prune(root, "")
}

// commandPaths lists every command in the tree, for the tests.
func commandPaths(root *cobra.Command) []string {
	var out []string
	var walk func(c *cobra.Command, prefix string)
	walk = func(c *cobra.Command, prefix string) {
		for _, sub := range c.Commands() {
			path := strings.TrimSpace(prefix + " " + sub.Name())
			out = append(out, path)
			walk(sub, path)
		}
	}
	walk(root, "")
	return out
}
