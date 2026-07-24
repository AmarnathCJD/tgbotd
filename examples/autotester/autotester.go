// Autonomous end-to-end tester: uses a gogram USER session to send /ping bursts
// to @Waw283Bot, then watches for responses via the same user session.
package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
)

// TGBOTD_STRING_SESSION: gogram StringSession for a real Telegram user account
// used to drive the bot under test. TGBOTD_TARGET_BOT overrides the target
// username (default @Waw283Bot).

type replyRecord struct {
	sentAt   time.Time
	seenAt   time.Time
	sentTxt  string
	replyTxt string
}

func main() {
	fmt.Println("[autotester] starting...")

	stringSession := os.Getenv("TGBOTD_STRING_SESSION")
	if stringSession == "" {
		fmt.Fprintln(os.Stderr, "TGBOTD_STRING_SESSION env var is required")
		os.Exit(2)
	}
	botUsername := os.Getenv("TGBOTD_TARGET_BOT")
	if botUsername == "" {
		botUsername = "Waw283Bot"
	}

	cli, err := telegram.NewClient(telegram.ClientConfig{
		AppID:         2040,
		AppHash:       "b18441a1ff607e10a989891a5462e627",
		StringSession: stringSession,
		LogLevel:      telegram.LogWarn,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "NewClient:", err)
		os.Exit(1)
	}
	if err := cli.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, "Connect:", err)
		os.Exit(1)
	}
	authed, _ := cli.IsAuthorized()
	if !authed {
		fmt.Fprintln(os.Stderr, "not authorized (bad string session)")
		os.Exit(1)
	}
	me := cli.Me()
	if me == nil {
		fmt.Fprintln(os.Stderr, "no Me()")
		os.Exit(1)
	}
	fmt.Printf("[autotester] connected as user %d (@%s)\n", me.ID, me.Username)

	bot, err := cli.ResolveUsername(botUsername)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ResolveUsername:", err)
		os.Exit(1)
	}
	var botID int64
	if v, ok := bot.(*telegram.UserObj); ok {
		botID = v.ID
	} else {
		fmt.Fprintf(os.Stderr, "unexpected peer type: %T\n", bot)
		os.Exit(1)
	}
	fmt.Printf("[autotester] bot resolved: id=%d\n", botID)

	var records []*replyRecord
	var recordsMu sync.Mutex
	var seenReplies atomic.Int32
	var expected atomic.Int32

	cli.AddRawHandler(nil, func(u telegram.Update, c *telegram.Client) error {
		fmt.Printf("[usr-raw] %T\n", u)
		return nil
	})
	cli.AddMessageHandler(telegram.OnMessage, func(m *telegram.NewMessage) error {
		if m == nil || m.Message == nil {
			return nil
		}
		fromID, _ := m.Message.FromID.(*telegram.PeerUser)
		var fid int64
		if fromID != nil {
			fid = fromID.UserID
		}
		fmt.Printf("[usr-msg] from=%d out=%v text=%q\n", fid, m.Message.Out, truncate(m.Message.Message, 60))
		if fromID != nil && fromID.UserID == botID {
			text := m.Message.Message
			recordsMu.Lock()
			for _, r := range records {
				if r.seenAt.IsZero() {
					r.seenAt = time.Now()
					r.replyTxt = text
					seenReplies.Add(1)
					fmt.Printf("[reply %d/%d] %q  (rt=%.2fs)\n",
						seenReplies.Load(), expected.Load(),
						truncate(text, 60),
						r.seenAt.Sub(r.sentAt).Seconds())
					break
				}
			}
			recordsMu.Unlock()
		}
		return nil
	})

	go cli.Idle()
	time.Sleep(2 * time.Second)

	runTest := func(name string, count int, delay time.Duration) {
		fmt.Printf("\n=== %s: %d pings, delay=%v ===\n", name, count, delay)
		expected.Store(int32(count))
		for i := 0; i < count; i++ {
			txt := fmt.Sprintf("/ping %s-%d", name, i)
			rec := &replyRecord{sentAt: time.Now(), sentTxt: txt}
			recordsMu.Lock()
			records = append(records, rec)
			recordsMu.Unlock()
			sent, err := cli.SendMessage(botID, txt)
			if err != nil {
				fmt.Printf("[send-err] %v\n", err)
			} else if sent != nil && sent.Message != nil {
				fmt.Printf("[sent] id=%d out=%v\n", sent.Message.ID, sent.Message.Out)
			} else {
				fmt.Printf("[sent] nil-msg\n")
			}
			time.Sleep(delay)
		}
		deadline := time.Now().Add(time.Duration(count*3) * time.Second)
		for time.Now().Before(deadline) && seenReplies.Load() < expected.Load() {
			time.Sleep(500 * time.Millisecond)
		}
		dumpResult(name, &recordsMu, &records)
		seenReplies.Store(0)
	}

	runTest("burst5", 5, 200*time.Millisecond)
	time.Sleep(3 * time.Second)
	runTest("space10", 10, 1500*time.Millisecond)

	fmt.Println("[autotester] done")
}

func dumpResult(name string, mu *sync.Mutex, records *[]*replyRecord) {
	mu.Lock()
	defer mu.Unlock()
	seen := 0
	total := len(*records)
	for _, r := range *records {
		if !r.seenAt.IsZero() {
			seen++
		}
	}
	fmt.Printf("\n=== %s RESULT: %d/%d replies ===\n", name, seen, total)
	for _, r := range *records {
		status := "MISS"
		rt := ""
		if !r.seenAt.IsZero() {
			status = "OK"
			rt = fmt.Sprintf(" (%.2fs)", r.seenAt.Sub(r.sentAt).Seconds())
		}
		fmt.Printf("  [%s] %s -> %s%s\n", status, r.sentTxt, truncate(r.replyTxt, 60), rt)
	}
	*records = nil
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
