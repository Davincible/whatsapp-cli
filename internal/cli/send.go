package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddmann/whatsapp-cli/internal/store"
	"github.com/eddmann/whatsapp-cli/internal/whatsapp"
)

var (
	sendFile    string
	sendCaption string
	sendReplyTo  string
	sendTextFile string
)

var sendCmd = &cobra.Command{
	Use:   "send <jid> <message>",
	Short: "Send a message",
	Long: `Send a text or media message to a JID.

Use 'whatsapp chats' to find the JID first.

Examples:
  whatsapp send 1234567890@s.whatsapp.net "Hello!"
  whatsapp send 1234567890@s.whatsapp.net --file photo.jpg --caption "Check this out"
  whatsapp send 1234567890@s.whatsapp.net "Reply text" --reply-to ABC123`,
	Args: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		textFile, _ := cmd.Flags().GetString("text-file")
		if textFile != "" {
			if len(args) != 1 {
				return fmt.Errorf("with --text-file, give only the jid")
			}
			return nil
		}
		if file != "" {
			if len(args) < 1 {
				return fmt.Errorf("requires at least 1 arg (jid)")
			}
			return nil
		}
		if len(args) < 2 {
			return fmt.Errorf("requires 2 args (jid and message)")
		}
		return nil
	},
	RunE: runSend,
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVar(&sendFile, "file", "", "Send a file (image, video, audio, document)")
	sendCmd.Flags().StringVar(&sendCaption, "caption", "", "Caption for media file")
	sendCmd.Flags().StringVar(&sendReplyTo, "reply-to", "", "Message ID to reply to")
	sendCmd.Flags().StringVar(&sendTextFile, "text-file", "",
		"send the exact UTF-8 contents of this file as the text (no shell quoting in between)")
}

func runSend(cmd *cobra.Command, args []string) error {
	jid := args[0]
	message := ""
	if len(args) > 1 {
		message = strings.Join(args[1:], " ")
	}
	if sendTextFile != "" {
		text, err := readTextFile(sendTextFile)
		if err != nil {
			return err
		}
		message = text
	}

	return WithConnection(func(db *store.DB, client *whatsapp.Client) error {
		var result *whatsapp.SendMessageResult
		var err error

		if sendFile != "" {
			result, err = client.SendMedia(jid, sendFile, sendCaption, sendReplyTo)
		} else {
			result, err = client.SendText(jid, message, sendReplyTo)
		}

		if err != nil {
			return fmt.Errorf("send failed: %w", err)
		}

		return OutputResult(store.SendResult{
			MessageID: result.MessageID,
			ChatJID:   result.ChatJID,
			Timestamp: result.Timestamp,
		}, fmt.Sprintf("Sent message %s", result.MessageID))
	})
}
