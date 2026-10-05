package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		path      string
		mediaType whatsmeow.MediaType
		mime      string
	}{
		{"photo.jpg", whatsmeow.MediaImage, "image/jpeg"},
		{"clip.MP4", whatsmeow.MediaVideo, "video/mp4"},
		{"note.ogg", whatsmeow.MediaAudio, "audio/ogg; codecs=opus"},
		{"track.mp3", whatsmeow.MediaAudio, "audio/mpeg"},
		{"deck.pdf", whatsmeow.MediaDocument, "application/pdf"},
		{"report.docx", whatsmeow.MediaDocument, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"rows.csv", whatsmeow.MediaDocument, "text/csv"},
		{"archive.tar.gz", whatsmeow.MediaDocument, "application/octet-stream"},
		{"noextension", whatsmeow.MediaDocument, "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			mediaType, mime := classify(tt.path)
			if mediaType != tt.mediaType {
				t.Errorf("media type = %v, want %v", mediaType, tt.mediaType)
			}
			if mime != tt.mime {
				t.Errorf("mime = %q, want %q", mime, tt.mime)
			}
		})
	}
}

func TestOwnRevoke(t *testing.T) {
	chat := types.NewJID("143902912843917", types.HiddenUserServer)
	msg := ownRevoke(chat, "3EB0ABC123")

	pm := msg.GetProtocolMessage()
	if pm == nil {
		t.Fatal("no protocol message")
	}
	if pm.GetType() != waE2E.ProtocolMessage_REVOKE {
		t.Errorf("type = %v, want REVOKE", pm.GetType())
	}
	key := pm.GetKey()
	if !key.GetFromMe() {
		t.Error("FromMe = false, want true: we only revoke our own messages")
	}
	if key.GetID() != "3EB0ABC123" {
		t.Errorf("ID = %q", key.GetID())
	}
	if key.GetRemoteJID() != "143902912843917@lid" {
		t.Errorf("RemoteJID = %q", key.GetRemoteJID())
	}
	if key.Participant != nil {
		t.Errorf("Participant = %q, want unset for a one-to-one chat", key.GetParticipant())
	}
}
