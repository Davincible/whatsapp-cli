package cli

import (
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"rsc.io/qr"
)

// The QR page lets someone link WhatsApp from a desktop app, where nobody sees
// the terminal: the login writes the current code into one HTML file, which
// reloads itself every two seconds, so the browser always shows the code that
// is valid now (WhatsApp rotates it about every 20 seconds). When the login
// ends, the same file shows the outcome and stops reloading.

const qrPageHead = `<!doctype html><html lang="en"><head><meta charset="utf-8">%s
<title>Link WhatsApp</title>
<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:3rem auto;padding:0 1rem;color:#111}
img{width:20rem;height:20rem;image-rendering:pixelated;display:block;margin:1.5rem 0}
p{line-height:1.5}</style></head><body>`

// writeQRPage writes the page showing code as a QR image.
func writeQRPage(path, code string) error {
	c, err := qr.Encode(code, qr.L)
	if err != nil {
		return fmt.Errorf("encode QR: %w", err)
	}
	png := base64.StdEncoding.EncodeToString(c.PNG())
	body := fmt.Sprintf(qrPageHead, `<meta http-equiv="refresh" content="2">`) +
		`<h1>Link WhatsApp</h1>` +
		`<p>On your phone, open WhatsApp, then <b>Settings</b>, <b>Linked devices</b>, <b>Link a device</b>, and point the camera at this code.</p>` +
		`<img alt="WhatsApp link code" src="data:image/png;base64,` + png + `">` +
		`<p>The code changes every few seconds; this page keeps up by itself.</p></body></html>`
	return writePage(path, body)
}

// writeStatusPage replaces the QR with an outcome and stops the reloading.
func writeStatusPage(path, title, detail string) error {
	body := fmt.Sprintf(qrPageHead, "") +
		`<h1>` + html.EscapeString(title) + `</h1><p>` + html.EscapeString(detail) + `</p></body></html>`
	return writePage(path, body)
}

// writePage replaces the file atomically, so a reload never sees half a page.
func writePage(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// openInBrowser opens path with the system's default handler.
func openInBrowser(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs) // #nosec G204 // a file path we wrote
	case "darwin":
		cmd = exec.Command("open", abs) // #nosec G204
	default:
		cmd = exec.Command("xdg-open", abs) // #nosec G204
	}
	return cmd.Start()
}
