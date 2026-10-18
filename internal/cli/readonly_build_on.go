//go:build readonly

package cli

// buildReadOnly is true in a binary built with `-tags readonly`: its command
// tree holds only the commands in readOnlyCommandPaths.
const buildReadOnly = true

func init() { pruneToReadOnly(rootCmd) }
