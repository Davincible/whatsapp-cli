package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQRPageReloadsWhileWaitingAndStopsAfter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "link.html")
	if err := writeQRPage(path, "2@abc,def,ghi"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `http-equiv="refresh"`) || !strings.Contains(string(b), "data:image/png;base64,") {
		t.Fatalf("QR page lacks the reload or the image: %s", b)
	}
	if err := writeStatusPage(path, "WhatsApp is connected", "<script>x</script>"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if strings.Contains(string(b), "refresh") || strings.Contains(string(b), "<script>") {
		t.Fatalf("status page still reloads or is not escaped: %s", b)
	}
}
