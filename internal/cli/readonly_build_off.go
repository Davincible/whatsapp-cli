//go:build !readonly

package cli

// buildReadOnly is false in a normal build, which keeps every command.
const buildReadOnly = false
