// Plain bot with tgbotd's EXACT push-to-buffer pattern + a concurrent HTTP server drain loop.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
)

// botToken is read from TGBOTD_TOKEN at startup.
var botToken = os.Getenv("TGBOTD_TOKEN")

type buf struct {
	mu    sync.Mutex
	cond  *sync.Cond
	items []telegram.Update
	next  int64
}

func newBuf() *buf {
	b := &buf{next: 1}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *buf) push(u telegram.Update) {
	b.mu.Lock()
	fmt.Fprintf(os.Stderr, "[push] %T\n", u)
	b.items = append(b.items, u)
	b.cond.Broadcast()
	b.mu.Unlock()
}

func (b *buf) wait(timeout time.Duration) []telegram.Update {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.items) > 0 {
		out := append([]telegram.Update(nil), b.items...)
		b.items = b.items[:0]
		return out
	}
	timer := time.AfterFunc(timeout, func() {
		b.mu.Lock()
		b.cond.Broadcast()
		b.mu.Unlock()
	})
	defer timer.Stop()
	for len(b.items) == 0 {
		b.cond.Wait()
	}
	out := append([]telegram.Update(nil), b.items...)
	b.items = b.items[:0]
	return out
}

func main() {
	os.MkdirAll("./data", 0o755)
	os.Remove("./data/plain_bot.session")
	os.Remove("./data/plain_bot.cache")
	cli, _ := telegram.NewClient(telegram.ClientConfig{
		AppID: 2040, AppHash: "b18441a1ff607e10a989891a5462e627",
		Session: "./data/plain_bot.session", SessionName: "plain_bot",
		Cache: telegram.NewCache("./data/plain_bot.cache"), LogLevel: telegram.LogWarn,
	})
	cli.Connect()
	if authed, _ := cli.IsAuthorized(); !authed {
		cli.LoginBot(botToken)
	}
	me := cli.Me()
	fmt.Printf("[plain_bot] connected as @%s\n", me.Username)

	b := newBuf()
	var rawCount atomic.Int32
	cli.AddRawHandler(nil, func(u telegram.Update, c *telegram.Client) error {
		n := rawCount.Add(1)
		fmt.Printf("[raw #%d] %T\n", n, u)
		b.push(u)
		return nil
	})

	// Simulate an HTTP client polling like Python testbot does
	mux := http.NewServeMux()
	mux.HandleFunc("/pull", func(w http.ResponseWriter, r *http.Request) {
		items := b.wait(25 * time.Second)
		out := make([]string, 0, len(items))
		for _, it := range items {
			out = append(out, reflect.TypeOf(it).String())
		}
		json.NewEncoder(w).Encode(out)
	})
	srv := &http.Server{Addr: "127.0.0.1:8082", Handler: mux}
	go srv.ListenAndServe()

	// Simulate testbot polling AND sending replies via RPC
	go func() {
		for {
			resp, err := http.Get("http://127.0.0.1:8082/pull")
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			var out []string
			json.NewDecoder(resp.Body).Decode(&out)
			resp.Body.Close()
			for range out {
				// Send RPC like tgbotd does after each update
				me := cli.Me()
				_ = me
			}
		}
	}()

	fmt.Println("[plain_bot] ready — send pings")
	// Graceful shutdown timer instead of Idle
	<-context.Background().Done()
	_ = srv
}
