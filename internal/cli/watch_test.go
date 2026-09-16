package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

func TestWatchEmitterFiltersToOneChat(t *testing.T) {
	var buf bytes.Buffer
	emit := newWatchEmitter(&buf, "123@g.us")
	body := "hello"
	emit(store.Message{ID: "A", ChatJID: "123@g.us", Content: &body, Timestamp: time.Unix(1, 0)})
	emit(store.Message{ID: "B", ChatJID: "999@g.us", Content: &body, Timestamp: time.Unix(2, 0)})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %q", len(lines), buf.String())
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("line is not JSON: %v", err)
	}
	if got["id"] != "A" || got["content"] != "hello" || got["chat_jid"] != "123@g.us" {
		t.Fatalf("unexpected event: %v", got)
	}
}

func TestWatchEmitterNoFilterPassesEverything(t *testing.T) {
	var buf bytes.Buffer
	emit := newWatchEmitter(&buf, "")
	emit(store.Message{ID: "A", ChatJID: "1@g.us"})
	emit(store.Message{ID: "B", ChatJID: "2@s.whatsapp.net"})
	if n := strings.Count(buf.String(), "\n"); n != 2 {
		t.Fatalf("want 2 lines, got %d", n)
	}
}
