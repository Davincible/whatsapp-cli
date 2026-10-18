//go:build windows

package sqlitedb

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// keyFileName holds the key sealed with DPAPI for the current Windows user:
// another account, or a copy of the disk, cannot open it, so it cannot open
// the store either.
const keyFileName = "store.key.dpapi"

func keyEntropy() []byte { return []byte("whatsapp-cli/store\x00") }

func blob(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]} // #nosec G115 // a key fits
}

func take(out *windows.DataBlob) []byte {
	// #nosec G103 // DPAPI returns a LocalAlloc'd buffer: copy it out, then free it
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...) // #nosec G103 // see above
}

func sealKey(b []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptProtectData(blob(b), nil, blob(keyEntropy()), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("DPAPI seal: %w", err)
	}
	return take(&out), nil
}

func unsealKey(b []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(blob(b), nil, blob(keyEntropy()), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, errors.New("this WhatsApp store was sealed by another Windows user or machine and cannot be opened here; log in again")
	}
	return take(&out), nil
}
