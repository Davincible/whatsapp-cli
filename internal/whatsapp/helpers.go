package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// parseJID parses a JID string into a types.JID.
func parseJID(jid string) (types.JID, error) {
	return types.ParseJID(jid)
}

// extractTextContent extracts text content from a WhatsApp message.
func extractTextContent(m *waE2E.Message) string {
	if m == nil {
		return ""
	}

	if t := m.GetConversation(); t != "" {
		return t
	}

	if et := m.GetExtendedTextMessage(); et != nil {
		return et.GetText()
	}

	if loc := m.GetLocationMessage(); loc != nil {
		return fmt.Sprintf("📍 Location: %.6f, %.6f", loc.GetDegreesLatitude(), loc.GetDegreesLongitude())
	}

	if contact := m.GetContactMessage(); contact != nil {
		name := contact.GetDisplayName()
		if name == "" {
			name = "Contact"
		}
		return fmt.Sprintf("👤 %s", name)
	}

	if sticker := m.GetStickerMessage(); sticker != nil {
		return "🎭 Sticker"
	}

	if liveLoc := m.GetLiveLocationMessage(); liveLoc != nil {
		return fmt.Sprintf("📍 Live Location: %.6f, %.6f", liveLoc.GetDegreesLatitude(), liveLoc.GetDegreesLongitude())
	}

	if poll := m.GetPollCreationMessage(); poll != nil {
		return fmt.Sprintf("📊 Poll: %s", poll.GetName())
	}

	if reaction := m.GetReactionMessage(); reaction != nil {
		return fmt.Sprintf("😊 Reaction: %s", reaction.GetText())
	}

	if m.GetProtocolMessage() != nil {
		return "🔧 System Message"
	}

	return ""
}

// mediaInfo is everything needed to download a media message after the fact.
//
// DirectPath is the locator whatsmeow actually downloads from: it resolves a
// current CDN host and appends the path to it. URL is the fully rendered URL
// WhatsApp sent, kept because it is all that rows written before direct_path
// existed have, and because it is useful to see.
type mediaInfo struct {
	MediaType     string
	Filename      string
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    uint64
}

// mediaAttachment is the subset of every waE2E media message this needs. Each
// of ImageMessage, VideoMessage, AudioMessage, DocumentMessage and
// StickerMessage satisfies it.
type mediaAttachment interface {
	GetURL() string
	GetDirectPath() string
	GetMediaKey() []byte
	GetFileSHA256() []byte
	GetFileEncSHA256() []byte
	GetFileLength() uint64
}

func newMediaInfo(mediaType, filename string, att mediaAttachment) mediaInfo {
	return mediaInfo{
		MediaType:     mediaType,
		Filename:      filename,
		URL:           att.GetURL(),
		DirectPath:    att.GetDirectPath(),
		MediaKey:      att.GetMediaKey(),
		FileSHA256:    att.GetFileSHA256(),
		FileEncSHA256: att.GetFileEncSHA256(),
		FileLength:    att.GetFileLength(),
	}
}

// extractMediaInfo extracts media information from a WhatsApp message.
func extractMediaInfo(m *waE2E.Message) mediaInfo {
	if m == nil {
		return mediaInfo{}
	}

	stamp := time.Now().Format("20060102_150405")

	if img := m.GetImageMessage(); img != nil {
		return newMediaInfo("image", fmt.Sprintf("image_%s.jpg", stamp), img)
	}

	if vid := m.GetVideoMessage(); vid != nil {
		return newMediaInfo("video", fmt.Sprintf("video_%s.mp4", stamp), vid)
	}

	if aud := m.GetAudioMessage(); aud != nil {
		return newMediaInfo("audio", fmt.Sprintf("audio_%s.ogg", stamp), aud)
	}

	if doc := m.GetDocumentMessage(); doc != nil {
		name := doc.GetFileName()
		if name == "" {
			name = fmt.Sprintf("document_%s", stamp)
		}
		return newMediaInfo("document", name, doc)
	}

	if sticker := m.GetStickerMessage(); sticker != nil {
		return newMediaInfo("sticker", fmt.Sprintf("sticker_%s.webp", stamp), sticker)
	}

	return mediaInfo{}
}

// classifyToWA converts media type string to WhatsApp MediaType.
func classifyToWA(t string) whatsmeow.MediaType {
	switch t {
	case "image":
		return whatsmeow.MediaImage
	case "video":
		return whatsmeow.MediaVideo
	case "audio":
		return whatsmeow.MediaAudio
	case "document":
		return whatsmeow.MediaDocument
	default:
		return whatsmeow.MediaDocument
	}
}

// extractDirectPathFromURL recovers a direct path from a rendered WhatsApp
// media URL. It is the fallback for rows stored before direct_path was
// persisted; prefer the stored direct_path when there is one.
//
// The query string is part of the direct path and must be kept. WhatsApp signs
// every path with oh= and oe= and the CDN answers 403 without them, and
// whatsmeow's DownloadMediaWithPath appends its own parameters with "&", not
// "?" — so a path stripped of its query yields a URL containing no "?" at all,
// where "&hash=..." lands inside the path. That stripping is what broke every
// media download once the CDN began enforcing the signature.
func extractDirectPathFromURL(rawURL string) string {
	_, path, found := strings.Cut(rawURL, ".net/")
	if !found {
		return rawURL
	}
	return "/" + path
}

// downloadable implements whatsmeow.DownloadableMessage interface.
type downloadable struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileLength    uint64
	FileSHA256    []byte
	FileEncSHA256 []byte
	MediaType     whatsmeow.MediaType
}

func (d *downloadable) GetDirectPath() string             { return d.DirectPath }
func (d *downloadable) GetURL() string                    { return d.URL }
func (d *downloadable) GetMediaKey() []byte               { return d.MediaKey }
func (d *downloadable) GetFileLength() uint64             { return d.FileLength }
func (d *downloadable) GetFileSHA256() []byte             { return d.FileSHA256 }
func (d *downloadable) GetFileEncSHA256() []byte          { return d.FileEncSHA256 }
func (d *downloadable) GetMediaType() whatsmeow.MediaType { return d.MediaType }
