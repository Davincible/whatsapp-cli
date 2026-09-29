package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

// loginWait is how long the status-reporting commands wait for the login
// handshake to finish after Connect returns. Without it they read IsLoggedIn
// before the server has answered and always report false, which made a working
// client look identical to one WhatsApp had rejected as outdated.
const loginWait = 10 * time.Second

// WithDB opens the database and runs the provided function.
// Syncs from WhatsApp first, unless --no-auto-sync was passed. See autosync.go.
func WithDB(fn func(*store.DB) error) error {
	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	// Auto-sync if needed
	if err := maybeAutoSync(db); err != nil {
		if fatal := fatalSyncError(err); fatal != nil {
			return fatal
		}
		fmt.Fprintf(os.Stderr, "Auto-sync warning: %v\n", err)
	}

	return fn(db)
}

// WithConnection opens the database, creates a WhatsApp client, verifies authentication,
// connects to WhatsApp, and runs the provided function.
// Syncs from WhatsApp first, unless --no-auto-sync was passed. See autosync.go.
func WithConnection(fn func(*store.DB, *whatsapp.Client) error) error {
	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if !client.IsAuthenticated() {
		return fmt.Errorf("not authenticated. Run 'whatsapp auth login' first")
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Disconnect()

	// Auto-sync if needed (using existing connection)
	if err := maybeAutoSyncWithClient(client, db); err != nil {
		if fatal := fatalSyncError(err); fatal != nil {
			return fatal
		}
		fmt.Fprintf(os.Stderr, "Auto-sync warning: %v\n", err)
	}

	return fn(db, client)
}

// fatalSyncError returns a non-nil error when a failed sync must stop the
// command rather than degrade into a stale read.
//
// The bar is deliberately narrow: only a refusal the server will keep repeating
// until something changes on this machine. A slow or unreachable network stays a
// warning, because the stored rows are still the best answer available and the
// warning says so. But an outdated client or an unlinked device will never
// recover on its own, and printing rows under a warning nobody reads is how a
// silently disconnected tool gets used to decide what to say to people.
//
// --no-auto-sync remains the way to read the store deliberately as-is.
func fatalSyncError(err error) error {
	var failure whatsapp.ConnectFailure
	if !errors.As(err, &failure) || !failure.Permanent() {
		return nil
	}
	return fmt.Errorf("%w\n\nto read the local store anyway, re-run with --no-auto-sync", failure)
}
