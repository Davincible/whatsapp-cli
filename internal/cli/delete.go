package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var deleteChat string

var deleteCmd = &cobra.Command{
	Use:   "delete <msg-id>...",
	Short: "Delete your own messages for everyone",
	Long: `Revoke one or more of your own messages, so they show as deleted for everyone
in the chat. Only your own messages can be deleted.

WhatsApp allows this for about two days after sending. The server accepts a
revoke for any ID, so success means the revoke was sent; read the chat back
(after a backfill) to see the message stored as "revoked".

Requires --chat. Use the exact JID the message was sent to.

Examples:
  whatsapp delete 3EB0ABC123 --chat 1234567890@s.whatsapp.net
  whatsapp delete 3EB0ABC123 3EB0DEF456 --chat 143902912843917@lid`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&deleteChat, "chat", "", "Chat JID (required)")
	_ = deleteCmd.MarkFlagRequired("chat")
}

// DeleteResult is one revoke attempt.
type DeleteResult struct {
	MessageID string `json:"message_id"`
	ChatJID   string `json:"chat_jid"`
	OK        bool   `json:"ok"`
	RevokeID  string `json:"revoke_id,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runDelete(cmd *cobra.Command, args []string) error {
	return WithConnection(func(_ *store.DB, client *whatsapp.Client) error {
		results := make([]DeleteResult, 0, len(args))
		failed := 0
		for _, id := range args {
			r := DeleteResult{MessageID: id, ChatJID: deleteChat}
			res, err := client.RevokeMessage(deleteChat, id)
			if err != nil {
				r.Error = err.Error()
				failed++
			} else {
				r.OK = true
				r.RevokeID = res.MessageID
				r.ChatJID = res.ChatJID
				r.Timestamp = res.Timestamp
			}
			results = append(results, r)
		}
		if err := OutputResult(results, fmt.Sprintf("Revoked %d of %d message(s)", len(args)-failed, len(args))); err != nil {
			return err
		}
		if failed > 0 {
			return fmt.Errorf("%d revoke(s) failed", failed)
		}
		return nil
	})
}
