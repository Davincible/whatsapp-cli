package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with WhatsApp via QR code and sync messages",
	Long: `Display a QR code in the terminal. Scan it with WhatsApp on your phone
(Settings → Linked Devices → Link a Device) to authenticate.

After authentication, an initial sync will start to download your message history.`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Disconnect and clear session",
	RunE:  runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection status and database stats",
	RunE:  runAuthStatus,
}

var (
	qrPage     string
	openQRPage bool
)

func init() {
	authLoginCmd.Flags().StringVar(&qrPage, "qr-page", "",
		"also write the QR code to this HTML page, which reloads itself as the code changes, then shows the outcome")
	authLoginCmd.Flags().BoolVar(&openQRPage, "open", false, "open the --qr-page in the default browser")
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	if err := EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Open messages database
	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	// Create WhatsApp client
	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		signal.Stop(sigChan)
		fmt.Fprintln(os.Stderr, "\nInterrupted, disconnecting...")
		client.Disconnect()
		cancel()
	}()

	// Connect with QR code
	fmt.Fprintln(os.Stderr, "Scan this QR code with WhatsApp (Settings → Linked Devices → Link a Device):")
	fmt.Fprintln(os.Stderr, "")

	var onCode func(string) error
	if qrPage != "" {
		opened := false
		onCode = func(code string) error {
			if err := writeQRPage(qrPage, code); err != nil {
				return err
			}
			if openQRPage && !opened {
				opened = true
				if err := openInBrowser(qrPage); err != nil {
					OutputWarning("Could not open %s: %v", qrPage, err)
				}
			}
			return nil
		}
	}

	if err := client.ConnectWithQR(ctx, onCode); err != nil {
		if qrPage != "" {
			_ = writeStatusPage(qrPage, "Not connected", err.Error())
		}
		return fmt.Errorf("connection failed: %w", err)
	}
	if qrPage != "" {
		_ = writeStatusPage(qrPage, "WhatsApp is connected",
			"Davincible Studio is reading your recent chats from your phone. You can close this tab.")
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Authenticated! Starting initial sync...")
	fmt.Fprintln(os.Stderr, "This may take a few minutes depending on your message history.")
	fmt.Fprintln(os.Stderr, "Press Ctrl+C to stop (you can resume sync later with 'whatsapp sync').")
	fmt.Fprintln(os.Stderr, "")

	// Wait for initial sync (with timeout)
	syncTimeout := 5 * time.Minute
	select {
	case <-ctx.Done():
		// User interrupted
	case <-client.SyncComplete:
		fmt.Fprintln(os.Stderr, "Initial sync complete.")
	case <-time.After(syncTimeout):
		fmt.Fprintln(os.Stderr, "Initial sync timeout reached. You can continue syncing with 'whatsapp sync'.")
	}

	client.Disconnect()

	// Output result
	user, device := client.GetDeviceID()
	return OutputResult(map[string]any{
		"user":   user,
		"device": device,
	}, fmt.Sprintf("Authenticated as %s (device %d)", user, device))
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseQuietly()

	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if client.IsAuthenticated() {
		// Connect briefly to unlink this device on WhatsApp's side
		if err := client.Connect(); err == nil {
			if err := client.Logout(); err != nil {
				OutputWarning("Logout failed: %v; remove the device from WhatsApp, Linked devices, on the phone", err)
			}
		} else {
			OutputWarning("Could not reach WhatsApp: %v; remove the device from WhatsApp, Linked devices, on the phone", err)
		}
		client.Disconnect()
	}

	// Remove the whole store: the session, the key, the message mirror and any
	// downloads. What stays is on the phone.
	db.CloseQuietly()
	if err := os.RemoveAll(GetStoreDir()); err != nil {
		return fmt.Errorf("failed to remove the store %s: %w", GetStoreDir(), err)
	}

	return OutputResult(map[string]any{"logged_out": true}, "Logged out; this computer's WhatsApp store is deleted")
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	status := store.ConnectionStatus{
		Connected: false,
		LoggedIn:  false,
	}

	db, err := store.Open(GetMessagesDBPath())
	if err != nil {
		// Database doesn't exist yet
		return Output(status)
	}
	defer db.CloseQuietly()

	client, err := whatsapp.New(db, GetStoreDir(), IsVerbose(), nil)
	if err != nil {
		return Output(status)
	}

	if client.IsAuthenticated() {
		// Try to connect to check actual status
		if err := client.Connect(); err == nil {
			client.WaitForLogin(loginWait)
			status.Connected = client.IsConnected()
			status.LoggedIn = client.IsLoggedIn()

			user, device := client.GetDeviceID()
			if user != "" {
				status.Device = &store.DeviceInfo{
					User:   user,
					Device: device,
				}
			}

			client.Disconnect()
		}
	}

	// Get database stats
	chatCount, _ := db.CountChats("")
	msgCount, _ := db.CountMessages()
	status.Database = &store.DBStats{
		Chats:    chatCount,
		Messages: msgCount,
	}

	return Output(status)
}
