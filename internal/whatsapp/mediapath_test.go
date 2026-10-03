package whatsapp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// composeCDNURL mirrors how whatsmeow's DownloadMediaWithPath builds the URL it
// fetches:
//
//	fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host, directPath, ...)
//
// Note the "&" before hash. whatsmeow assumes the direct path already carries
// its own query string, and that assumption is the whole point of these tests.
func composeCDNURL(directPath string) string {
	return fmt.Sprintf("https://mmg.whatsapp.net%s&hash=%s&mms-type=%s&__wa-mms=",
		directPath, "We0GVdQEc6s5TZ3xuGXV", "audio")
}

// TestDirectPathComposesIntoAFetchableURL pins the contract between our derived
// direct path and whatsmeow's URL construction.
//
// It exists because extractDirectPathFromURL used to drop the query string. The
// result parsed as a URL and looked plausible, but WhatsApp signs each path with
// oh= and oe=, and without them the CDN answers 403. Worse, with no "?" left in
// the path, whatsmeow's own "&hash=..." landed inside the path rather than the
// query.
//
// Every media download through this tool failed with 403 once the CDN began
// enforcing that signature. Asserting on the composed URL rather than on the
// returned string is deliberate: a test that only compared the path against a
// literal would have passed throughout the outage.
func TestDirectPathComposesIntoAFetchableURL(t *testing.T) {
	const signedQuery = "ccb=11-4&oh=01_Q5Aa5gHrEWErZ5Grfq1vQ&oe=6AE784F8&_nc_sid=5e03e0&mms3=true"
	const encPath = "/v/t62.7117-24/832665140_1087022260737384_n.enc"

	directPath := extractDirectPathFromURL("https://mmg.whatsapp.net" + encPath + "?" + signedQuery)

	parsed, err := url.Parse(composeCDNURL(directPath))
	if err != nil {
		t.Fatalf("composed URL does not parse: %v", err)
	}

	if parsed.Path != encPath {
		t.Errorf("path = %q, want %q — whatsmeow's parameters leaked into the path", parsed.Path, encPath)
	}

	q := parsed.Query()
	// The signature WhatsApp requires. Without these the CDN returns 403.
	for _, key := range []string{"oh", "oe", "ccb", "_nc_sid", "mms3"} {
		if !q.Has(key) {
			t.Errorf("query is missing %q, which the CDN requires to serve the object", key)
		}
	}
	// whatsmeow's own parameters have to land in the query, not the path.
	for _, key := range []string{"hash", "mms-type", "__wa-mms"} {
		if !q.Has(key) {
			t.Errorf("query is missing %q: whatsmeow appends it with \"&\", so the "+
				"direct path must already end in a query string", key)
		}
	}
}

func TestExtractDirectPathFromURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "full CDN URL keeps its signed query",
			in:   "https://mmg.whatsapp.net/v/t62.7117-24/abc_n.enc?ccb=11-4&oh=sig&oe=6AE784F8",
			want: "/v/t62.7117-24/abc_n.enc?ccb=11-4&oh=sig&oe=6AE784F8",
		},
		{
			name: "a bare direct path is already what we want",
			in:   "/v/t62.7117-24/abc_n.enc?ccb=11-4&oh=sig",
			want: "/v/t62.7117-24/abc_n.enc?ccb=11-4&oh=sig",
		},
		{
			name: "unrecognised value is passed through rather than mangled",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDirectPathFromURL(tt.in); got != tt.want {
				t.Errorf("extractDirectPathFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestExtractMediaInfoPersistsDirectPath checks that the authoritative
// direct_path is carried off the message rather than re-derived from the URL.
// WhatsApp leaves URL empty on some media, where direct path is all there is.
func TestExtractMediaInfoPersistsDirectPath(t *testing.T) {
	const directPath = "/v/t62.7117-24/abc_n.enc?ccb=11-4&oh=sig&oe=6AE784F8"

	audio := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(""), // WhatsApp does omit this
			DirectPath:    proto.String(directPath),
			MediaKey:      []byte("key"),
			FileSHA256:    []byte("sha"),
			FileEncSHA256: []byte("enc"),
			FileLength:    proto.Uint64(729105),
			PTT:           proto.Bool(true),
		},
	}

	got := extractMediaInfo(audio)

	if got.MediaType != "audio" {
		t.Errorf("MediaType = %q, want \"audio\"", got.MediaType)
	}
	if got.DirectPath != directPath {
		t.Errorf("DirectPath = %q, want %q", got.DirectPath, directPath)
	}
	if got.FileLength != 729105 {
		t.Errorf("FileLength = %d, want 729105", got.FileLength)
	}
}

// TestUniqueMediaFilenameKeepsCollidingMessagesApart covers the second way these
// downloads lost voice notes.
//
// Stored filenames are stamped with the sync clock, not the message, so ten
// voice notes received overnight shared four names. Each download wrote to the
// same path, overwrote the previous file, and reported success.
func TestUniqueMediaFilenameKeepsCollidingMessagesApart(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.CloseQuietly()

	const chatJID = "31634727863-1474609951@g.us"
	if _, err := db.Messages.Exec(`INSERT INTO chats (jid, name) VALUES (?, ?)`, chatJID, "Self notes"); err != nil {
		t.Fatalf("insert chat: %v", err)
	}

	// Three voice notes the sync named identically, plus one named uniquely.
	rows := []struct{ id, filename string }{
		{"AC50A2012CC55E40B24B51F265101D50", "audio_20261003_150932.ogg"},
		{"AC7E2FFC93047C753ADCD214E05F9027", "audio_20261003_150932.ogg"},
		{"ACB4E8295D205BAFB1C21E3D7B0BF779", "audio_20261003_150932.ogg"},
		{"AC729FD0FCA62EFDDD145685C7C49FB7", "audio_20261003_150952.ogg"},
	}
	for _, r := range rows {
		if _, err := db.Messages.Exec(
			`INSERT INTO messages (id, chat_jid, sender, timestamp, is_from_me, media_type, filename)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			r.id, chatJID, "31634727863", time.Now(), true, "audio", r.filename,
		); err != nil {
			t.Fatalf("insert message %s: %v", r.id, err)
		}
	}

	c := &Client{Store: db}

	// The shared name must resolve to a distinct path per message.
	seen := map[string]string{}
	for _, r := range rows[:3] {
		got := c.uniqueMediaFilename(r.id, chatJID, r.filename)
		if got == r.filename {
			t.Errorf("%s kept the shared name %q, so it would overwrite a sibling", r.id, got)
		}
		if !strings.HasSuffix(got, ".ogg") {
			t.Errorf("%s produced %q, which lost its extension", r.id, got)
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("%s and %s both resolved to %q", prev, r.id, got)
		}
		seen[got] = r.id
	}

	// A name only one message claims is left alone.
	unique := rows[3]
	if got := c.uniqueMediaFilename(unique.id, chatJID, unique.filename); got != unique.filename {
		t.Errorf("uncontested name = %q, want it left as %q", got, unique.filename)
	}
}
