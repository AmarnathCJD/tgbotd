// Standalone latency benchmark: tgbotd vs api.telegram.org.
//
// Runs a small suite of Bot API methods against both endpoints, N times each
// (serial), and reports median / p95 / mean.
//
//	go run ./examples/bench -token=... -local=http://127.0.0.1:8081 -n=20
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {
	token := flag.String("token", os.Getenv("TGBOTD_BENCH_TOKEN"), "bot token")
	local := flag.String("local", "http://127.0.0.1:8081", "tgbotd base URL")
	upstream := flag.String("upstream", "https://api.telegram.org", "upstream Bot API base URL")
	n := flag.Int("n", 20, "iterations per method")
	parallel := flag.Int("parallel", 1, "concurrent requests per iteration (throughput mode)")
	warmup := flag.Int("warmup", 3, "warmup iterations (not counted)")
	flag.Parse()

	if *token == "" {
		fmt.Fprintln(os.Stderr, "-token or TGBOTD_BENCH_TOKEN required")
		os.Exit(2)
	}

	type call struct {
		name  string
		body  string
	}
	suite := []call{
		{"getMe", ``},
		{"getWebhookInfo", ``},
		{"getUpdates", `{"timeout":0,"limit":1}`},
		{"resolveUsername", `{"username":"telegram"}`}, // tgbotd-only extension; will 404 on upstream
	}

	fmt.Printf("bench: n=%d parallel=%d warmup=%d\n", *n, *parallel, *warmup)
	fmt.Printf("  local    = %s\n", *local)
	fmt.Printf("  upstream = %s\n\n", *upstream)

	header := fmt.Sprintf("%-18s | %-8s | %8s %8s %8s %8s %8s | %s",
		"method", "target", "min", "p50", "mean", "p95", "max", "errors")
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))

	for _, c := range suite {
		for _, target := range []struct {
			name string
			base string
		}{{"local", *local}, {"upstream", *upstream}} {
			// warmup
			for i := 0; i < *warmup; i++ {
				_, _ = timedCall(target.base, *token, c.name, c.body)
			}
			// measure
			samples := make([]time.Duration, 0, *n**parallel)
			errors := 0
			var mu sync.Mutex
			for i := 0; i < *n; i++ {
				var wg sync.WaitGroup
				for k := 0; k < *parallel; k++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						d, err := timedCall(target.base, *token, c.name, c.body)
						mu.Lock()
						samples = append(samples, d)
						if err != nil {
							errors++
						}
						mu.Unlock()
					}()
				}
				wg.Wait()
			}
			r := summarize(samples)
			fmt.Printf("%-18s | %-8s | %8s %8s %8s %8s %8s | %d\n",
				c.name, target.name,
				r.min, r.p50, r.mean, r.p95, r.max, errors)
		}
	}
}

func timedCall(base, token, method, body string) (time.Duration, error) {
	url := fmt.Sprintf("%s/bot%s/%s", base, token, method)
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest("POST", url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest("POST", url, nil)
	}
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return time.Since(start), err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	d := time.Since(start)
	// Only count OK responses (200-family). Everything else is an error for
	// bench purposes.
	var wrapper struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(b, &wrapper); err != nil || !wrapper.OK {
		return d, fmt.Errorf("not ok")
	}
	return d, nil
}

type report struct {
	min, p50, p95, mean, max time.Duration
}

func summarize(s []time.Duration) report {
	if len(s) == 0 {
		return report{}
	}
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	var sum time.Duration
	for _, d := range s {
		sum += d
	}
	pct := func(p float64) time.Duration {
		i := int(float64(len(s)) * p)
		if i >= len(s) {
			i = len(s) - 1
		}
		return s[i]
	}
	return report{
		min:  s[0],
		p50:  pct(0.5),
		p95:  pct(0.95),
		mean: sum / time.Duration(len(s)),
		max:  s[len(s)-1],
	}
}
