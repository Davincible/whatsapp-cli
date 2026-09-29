// Command waversion prints the WhatsApp client version this build reports to
// the server.
//
// It exists because that version is the thing WhatsApp expires, and it is
// otherwise buried in a whatsmeow source file. `make upgrade` prints it before
// and after a bump so the upgrade can be seen to have done something.
package main

import (
	"fmt"

	"go.mau.fi/whatsmeow/store"
)

func main() {
	fmt.Println(store.GetWAVersion().String())
}
