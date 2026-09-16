package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var watchCmd = &cobra.Command{
	Use:   "watch [jid]",
	Short: "Stream incoming messages as JSON lines",
	Long: `Connect to WhatsApp and print every real-time message as one JSON object per
line, in the same shape as 'messages -f jsonl'. Messages are also stored, exactly
as 'sync --follow' does. Give a chat JID to print only that chat.

Only one process can hold the WhatsApp connection. While watch is running, read
with --no-auto-sync so a read does not open a second connection.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	filter := ""
	if len(args) == 1 {
		filter = args[0]
	}

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

	client.OnMessage = newWatchEmitter(os.Stdout, filter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		signal.Stop(sigChan)
		cancel()
	}()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	if filter != "" {
		fmt.Fprintf(os.Stderr, "Watching %s for new messages (Ctrl-C to stop)…\n", filter)
	} else {
		fmt.Fprintln(os.Stderr, "Watching all chats for new messages (Ctrl-C to stop)…")
	}

	<-ctx.Done()
	client.Disconnect()
	return nil
}

// newWatchEmitter returns the OnMessage callback for watch: it writes each
// message from the filtered chat (every chat when filter is empty) as one JSON
// line. Writes are serialised because whatsmeow may deliver events concurrently.
func newWatchEmitter(w io.Writer, filter string) func(store.Message) {
	var mu sync.Mutex
	enc := json.NewEncoder(w)
	return func(m store.Message) {
		if filter != "" && m.ChatJID != filter {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_ = enc.Encode(m)
	}
}
