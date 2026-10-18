package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waHistorySync "go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types/events"
)

const syncCompletionSettleDelay = 5 * time.Second

// registerHandlers registers event handlers for WhatsApp events.
func (c *Client) registerHandlers() {
	c.WA.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			c.handleMessage(v)
		case *events.HistorySync:
			result := c.handleHistorySync(v)
			if v.Data != nil && v.Data.GetSyncType() == waHistorySync.HistorySync_ON_DEMAND {
				c.completeBackfillRequest(result)
			}
			// Check if sync is complete (progress == 100)
			if v.Data != nil && v.Data.Progress != nil && *v.Data.Progress >= 100 {
				c.Logger.Info("history sync complete")
				c.backfillChatNames()
				c.signalSyncCompleteAfterSettleDelay()
			}
		case *events.OfflineSyncCompleted:
			c.backfillChatNames()
			count, _ := c.Store.CountMessages()
			if count > 0 {
				c.signalSyncCompleteAfterSettleDelay()
			}
		case *events.ChatPresence:
			c.handleChatPresence(v)
		case *events.Connected:
			c.Logger.Info("connected to WhatsApp")
		}

		// Login refusals arrive as several different event types, so they are
		// matched separately rather than as cases above. See connectfail.go.
		if failure, ok := connectFailureFromEvent(evt); ok {
			c.publishConnectFailure(failure)
		}
	})
}

func (c *Client) signalSyncCompleteAfterSettleDelay() {
	c.syncCompleteMu.Lock()
	defer c.syncCompleteMu.Unlock()

	if c.syncCompleteTimer != nil {
		c.syncCompleteTimer.Stop()
	}

	c.syncCompleteTimer = time.AfterFunc(syncCompletionSettleDelay, func() {
		if !c.WA.IsConnected() {
			return
		}

		select {
		case c.SyncComplete <- struct{}{}:
		default:
		}
	})
}

// ConnectWithQR connects to WhatsApp, displaying a QR code if needed. Each new
// code is drawn on stderr and, when onCode is set, handed to it too (the QR page
// for a desktop app). It returns once paired, or an error when the codes run
// out or WhatsApp refuses the pairing.
func (c *Client) ConnectWithQR(ctx context.Context, onCode func(code string) error) error {
	if c.WA.Store.ID != nil {
		return c.WA.Connect()
	}
	qrChan, err := c.WA.GetQRChannel(ctx)
	if err != nil {
		return err
	}
	if err := c.WA.Connect(); err != nil {
		return err
	}
	for evt := range qrChan {
		switch {
		case evt.Event == "code":
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stderr)
			if onCode != nil {
				if err := onCode(evt.Code); err != nil {
					return err
				}
			}
		case evt == whatsmeow.QRChannelSuccess:
			return nil
		case evt == whatsmeow.QRChannelTimeout:
			return errors.New("the QR code expired before it was scanned; run login again")
		case evt == whatsmeow.QRChannelClientOutdated:
			return errors.New("WhatsApp refused this client version (client outdated); update the whatsapp tool")
		case evt == whatsmeow.QRChannelScannedWithoutMultidevice:
			return errors.New("the phone that scanned has multi-device turned off; turn it on in WhatsApp and run login again")
		case evt.Error != nil:
			return fmt.Errorf("pairing failed: %w", evt.Error)
		default:
			return fmt.Errorf("pairing failed: %s", evt.Event)
		}
	}
	return ctx.Err()
}

// Connect connects to WhatsApp without QR (requires existing session).
func (c *Client) Connect() error {
	return c.WA.Connect()
}

// publishConnectFailure records a refusal from the server and publishes it, so a
// waiting command can fail immediately with the real reason instead of sitting
// out the full auto-sync timeout and then blaming the store.
func (c *Client) publishConnectFailure(failure ConnectFailure) {
	c.connectFailMu.Lock()
	c.connectFailure = &failure
	c.connectFailMu.Unlock()

	c.Logger.Error("connection refused by WhatsApp",
		"reason", failure.Message, "code", int(failure.Reason))

	select {
	case c.ConnectFailed <- failure:
	default:
	}
}
