package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	tgAPI   = "http://127.0.0.1:8081" // point at local tgbotd
	token   = "TESTBOT_TOKEN_HERE"
	hookURL = "http://127.0.0.1:9999/hook"
	secret  = "s3cr3t-abc-123"
	N       = 4
)

type got struct {
	body   []byte
	secret string
	at     time.Time
}

func main() {
	var (
		mu   sync.Mutex
		recv []got
	)
	srv := &http.Server{Addr: "127.0.0.1:9999"}
	http.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		recv = append(recv, got{b, r.Header.Get("X-Telegram-Bot-Api-Secret-Token"), time.Now()})
		mu.Unlock()
		w.WriteHeader(200)
	})
	go srv.ListenAndServe()
	defer srv.Shutdown(context.Background())

	call := func(m string, q url.Values) {
		resp, err := http.Get(fmt.Sprintf("%s/bot%s/%s?%s", tgAPI, token, m, q.Encode()))
		if err != nil {
			fail("call %s: %v", m, err)
		}
		resp.Body.Close()
	}
	defer call("deleteWebhook", url.Values{})

	call("setWebhook", url.Values{"url": {hookURL}, "secret_token": {secret}})
	fmt.Println("[hook] set")
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command("go", "run", "../autotester_short")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	sentAt := time.Now()
	if err := cmd.Run(); err != nil {
		fail("autotester: %v", err)
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(recv)
		mu.Unlock()
		if n >= N {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\n=== got %d/%d in %v ===\n", len(recv), N, time.Since(sentAt))
	if len(recv) < N {
		fail("timeout: only %d/%d", len(recv), N)
	}
	type parsed struct {
		UpdateID int64 `json:"update_id"`
		Message  *struct {
			Text string `json:"text"`
			From *struct{ ID int64 } `json:"from"`
			Chat *struct{ ID int64 } `json:"chat"`
		} `json:"message"`
	}
	ps := make([]parsed, len(recv))
	for i, g := range recv {
		if g.secret != secret {
			fail("#%d bad secret %q", i, g.secret)
		}
		if err := json.Unmarshal(g.body, &ps[i]); err != nil {
			fail("#%d json: %v", i, err)
		}
		p := ps[i]
		if p.UpdateID == 0 || p.Message == nil || p.Message.Text == "" ||
			p.Message.From == nil || p.Message.Chat == nil {
			fail("#%d schema: %s", i, g.body)
		}
		fmt.Printf("  [OK] uid=%d text=%q\n", p.UpdateID, p.Message.Text)
	}
	if !sort.SliceIsSorted(ps, func(i, j int) bool { return ps[i].UpdateID < ps[j].UpdateID }) {
		fail("delivery order != update_id order")
	}
	for i, p := range ps {
		want := fmt.Sprintf("/ping short-%d", i)
		if !strings.Contains(p.Message.Text, want) {
			fail("#%d text=%q want contains %q", i, p.Message.Text, want)
		}
	}
	fmt.Println("PASS")
}

func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+f+"\n", a...)
	http.Get(fmt.Sprintf("%s/bot%s/deleteWebhook", tgAPI, token))
	os.Exit(1)
}
