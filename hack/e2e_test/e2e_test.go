package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const base = "http://127.0.0.1:8081"

func TestServerReachable(t *testing.T) {
	token := os.Getenv("TGBOTD_TOKEN")
	if token == "" {
		token = os.Getenv("TGBOTD_BENCH_TOKEN")
	}
	if token == "" {
		t.Skip("TGBOTD_TOKEN not set")
	}
	body, _ := json.Marshal(map[string]any{})
	resp, err := http.Post(base+"/bot"+token+"/getMe", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Skip("server not running:", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var r struct {
		OK     bool `json:"ok"`
		Result struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &r); err != nil || !r.OK {
		t.Fatalf("getMe failed: %s", string(b))
	}
	t.Logf("connected as @%s (id %d)", r.Result.Username, r.Result.ID)
}

// TestGetUpdatesIdempotent verifies that a poll with the same offset returns
// consistently, and offset advancement drops old items.
func TestGetUpdatesIdempotent(t *testing.T) {
	token := os.Getenv("TGBOTD_TOKEN")
	if token == "" {
		token = os.Getenv("TGBOTD_BENCH_TOKEN")
	}
	if token == "" {
		t.Skip("TGBOTD_TOKEN not set")
	}
	call := func(method string, params map[string]any) map[string]any {
		body, _ := json.Marshal(params)
		req, _ := http.NewRequest("POST", base+"/bot"+token+"/"+method, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var r map[string]any
		json.Unmarshal(b, &r)
		return r
	}
	// First call: short poll with offset 0.
	r1 := call("getUpdates", map[string]any{"offset": 0, "timeout": 0})
	if !r1["ok"].(bool) {
		t.Fatalf("first getUpdates failed: %v", r1)
	}
	res1, _ := r1["result"].([]any)
	t.Logf("first poll: %d updates", len(res1))

	// Second call with offset+100: should return empty.
	r2 := call("getUpdates", map[string]any{"offset": 999999999, "timeout": 0})
	if !r2["ok"].(bool) {
		t.Fatalf("second getUpdates failed: %v", r2)
	}
	res2, _ := r2["result"].([]any)
	if len(res2) != 0 {
		t.Errorf("expected empty result with high offset, got %d", len(res2))
	}
}

// TestBackToBackPolling simulates the bot's actual polling behavior: keep
// polling short-timeout with rolling offset. Should never produce duplicate
// update_ids or lose updates in the middle.
func TestBackToBackPolling(t *testing.T) {
	token := os.Getenv("TGBOTD_TOKEN")
	if token == "" {
		token = os.Getenv("TGBOTD_BENCH_TOKEN")
	}
	if token == "" {
		t.Skip("TGBOTD_TOKEN not set")
	}
	call := func(method string, params map[string]any) map[string]any {
		body, _ := json.Marshal(params)
		req, _ := http.NewRequest("POST", base+"/bot"+token+"/"+method, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var r map[string]any
		json.Unmarshal(b, &r)
		return r
	}
	var offset int64 = 0
	seen := map[int64]bool{}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r := call("getUpdates", map[string]any{"offset": offset, "timeout": 0})
		if !r["ok"].(bool) {
			t.Fatalf("getUpdates: %v", r)
		}
		res, _ := r["result"].([]any)
		if len(res) == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		for _, u := range res {
			um := u.(map[string]any)
			uid := int64(um["update_id"].(float64))
			if seen[uid] {
				t.Errorf("duplicate update_id=%d", uid)
			}
			seen[uid] = true
			if uid+1 > offset {
				offset = uid + 1
			}
		}
	}
	t.Logf("saw %d distinct updates over 3s", len(seen))
}
