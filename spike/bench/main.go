// SPIKE (throwaway): measures the message store on whichever SQLite driver this
// checkout builds with. Synthetic data only; never point it at a real store.
//
//	go run ./spike/bench <empty dir> fill      # 200,000 messages in 300 chats
//	go run ./spike/bench <dir> read            # open, list one chat, two searches
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/eddmann/whatsapp-cli/internal/store"
)

var words = []string{"invoice", "launch", "meeting", "tomorrow", "budget", "video", "edit",
	"draft", "client", "payment", "deadline", "photo", "dinner", "flight", "contract", "review",
	"thanks", "call", "update", "schedule", "price", "shoot", "studio", "render", "story"}

func main() {
	dir, mode := os.Args[1], os.Args[2]
	path := filepath.Join(dir, "messages.db")
	t0 := time.Now()
	db, err := store.Open(path)
	if err != nil {
		fmt.Println("open error:", err)
		os.Exit(1)
	}
	fmt.Printf("open: %v\n", time.Since(t0).Round(time.Millisecond))
	switch mode {
	case "fill":
		r := rand.New(rand.NewSource(1))
		t := time.Now()
		tx, _ := db.Messages.Begin()
		for c := 0; c < 300; c++ {
			_, _ = tx.Exec(`INSERT INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)`,
				fmt.Sprintf("%d@s.whatsapp.net", 31600000000+c), fmt.Sprintf("Chat %d", c), time.Now())
		}
		for i := 0; i < 200000; i++ {
			n := 4 + r.Intn(16)
			text := ""
			for w := 0; w < n; w++ {
				text += words[r.Intn(len(words))] + " "
			}
			if i == 123456 {
				text += "zebracanary"
			}
			c := r.Intn(300)
			_, err := tx.Exec(`INSERT INTO messages (id, chat_jid, sender, content, timestamp, is_from_me) VALUES (?, ?, ?, ?, ?, ?)`,
				fmt.Sprintf("M%07d", i), fmt.Sprintf("%d@s.whatsapp.net", 31600000000+c), "x", text,
				time.Unix(1700000000+int64(i)*60, 0), i%3 == 0)
			if err != nil {
				fmt.Println("insert error:", err)
				os.Exit(1)
			}
		}
		if err := tx.Commit(); err != nil {
			fmt.Println("commit error:", err)
			os.Exit(1)
		}
		fmt.Printf("insert 200k: %v\n", time.Since(t).Round(time.Millisecond))
	case "read":
		t := time.Now()
		ms, err := db.ListMessages(store.ListMessagesOptions{ChatJID: "31600000042@s.whatsapp.net", Limit: 50})
		fmt.Printf("list 50 in one chat: %d rows, err=%v, %v\n", len(ms), err, time.Since(t).Round(time.Millisecond))
		t = time.Now()
		ms, err = db.SearchMessages(store.SearchMessagesOptions{Query: "invoice deadline", Limit: 50})
		fmt.Printf("search common words: %d rows, err=%v, %v\n", len(ms), err, time.Since(t).Round(time.Millisecond))
		t = time.Now()
		ms, err = db.SearchMessages(store.SearchMessagesOptions{Query: "zebracanary", Limit: 50})
		fmt.Printf("search rare word: %d rows, err=%v, %v\n", len(ms), err, time.Since(t).Round(time.Millisecond))
	}
	db.CloseQuietly()
	if fi, err := os.Stat(path); err == nil {
		fmt.Printf("file size: %.1f MB\n", float64(fi.Size())/1e6)
	}
}
