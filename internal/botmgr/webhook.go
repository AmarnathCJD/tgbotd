package botmgr

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

type WebhookConfig struct {
	URL         string
	Secret      string
	IP          string
	MaxConns    int
	Allowed     []string
	DropPending bool
}

type deliverer struct {
	bot    *Bot
	cfg    WebhookConfig
	client *http.Client
	stopCh chan struct{}
	once   sync.Once
}

func (b *Bot) StartWebhook(cfg WebhookConfig) {
	b.stopWebhook()
	if cfg.URL == "" {
		return
	}
	d := &deliverer{
		bot: b,
		cfg: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        16,
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
		stopCh: make(chan struct{}),
	}
	b.setDeliverer(d)
	go d.run()
}

func (b *Bot) stopWebhook() {
	if d := b.takeDeliverer(); d != nil {
		d.once.Do(func() { close(d.stopCh) })
	}
}

var (
	delivererMu  sync.Mutex
	delivererMap = map[*Bot]*deliverer{}
)

func (b *Bot) setDeliverer(d *deliverer) {
	delivererMu.Lock()
	delivererMap[b] = d
	delivererMu.Unlock()
}

func (b *Bot) takeDeliverer() *deliverer {
	delivererMu.Lock()
	d := delivererMap[b]
	delete(delivererMap, b)
	delivererMu.Unlock()
	return d
}

func stopAllDeliverers() {
	delivererMu.Lock()
	dels := make([]*deliverer, 0, len(delivererMap))
	for _, d := range delivererMap {
		dels = append(dels, d)
	}
	delivererMap = map[*Bot]*deliverer{}
	delivererMu.Unlock()
	for _, d := range dels {
		d.once.Do(func() { close(d.stopCh) })
	}
}

// maxDeliveryAttempts caps how many times a single update is retried before
// the deliverer gives up and advances past it. Prevents a broken endpoint
// from freezing the entire update pipeline on one bad item.
const maxDeliveryAttempts = 8

func (d *deliverer) run() {
	backoff := time.Second
	// Per-update attempt counter: keyed by update ID so we drop stuck items
	// after maxDeliveryAttempts rather than blocking every subsequent update.
	attempts := map[int64]int{}
	for {
		select {
		case <-d.stopCh:
			return
		default:
		}
		release := d.bot.Updates.TakeLock()
		items := d.bot.Updates.Wait(context.Background(), 0, 100, 30*time.Second)
		if len(items) == 0 {
			release()
			continue
		}
		tctx := d.bot.BuildTranslateContext()
		var lastAcked int64
		anyFailure := false
		for _, item := range items {
			select {
			case <-d.stopCh:
				if lastAcked > 0 {
					d.bot.Updates.Ack(lastAcked)
				}
				release()
				return
			default:
			}
			bu := tlate.UpdateToBotAPI(item.Update, tctx)
			if bu == nil {
				lastAcked = item.ID + 1
				delete(attempts, item.ID)
				continue
			}
			bu.UpdateID = item.ID
			if d.deliverOne(bu) {
				lastAcked = item.ID + 1
				delete(attempts, item.ID)
				backoff = time.Second
				continue
			}
			// Delivery failed. Bump attempt count; if we've tried too many
			// times, log-and-skip so subsequent updates aren't held hostage.
			attempts[item.ID]++
			if attempts[item.ID] >= maxDeliveryAttempts {
				d.bot.log.Warn("webhook: dropping update after max attempts",
					"update_id", item.ID, "attempts", attempts[item.ID])
				lastAcked = item.ID + 1
				delete(attempts, item.ID)
				continue
			}
			anyFailure = true
			break
		}
		if lastAcked > 0 {
			d.bot.Updates.Ack(lastAcked)
		}
		release()
		if anyFailure {
			select {
			case <-d.stopCh:
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > 300*time.Second {
				backoff = 300 * time.Second
			}
		}
	}
}

func (d *deliverer) deliverOne(u *botapi.Update) bool {
	body, err := json.Marshal(u)
	if err != nil {
		return false
	}
	req, err := http.NewRequest("POST", d.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	if d.cfg.Secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", d.cfg.Secret)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return false
	}
	io.CopyN(io.Discard, resp.Body, 1<<20)
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// buildTranslateContext extracts the users/chats maps from gogram's peer cache
// so translation isn't lossy. Returns a snapshot; safe to hand to translator.
func (b *Bot) BuildTranslateContext() *tlate.TranslateContext {
	tctx := &tlate.TranslateContext{
		Users: map[int64]*telegram.UserObj{},
		Chats: map[int64]telegram.Chat{},
	}
	if me := b.Client.Me(); me != nil {
		tctx.SelfID = me.ID
		tctx.Users[me.ID] = me
	}
	tctx.UserLookup = func(id int64) *telegram.UserObj {
		if u, err := b.Client.GetUser(id); err == nil {
			return u
		}
		return nil
	}
	tctx.ChatLookup = func(id int64) telegram.Chat {
		if ch, err := b.Client.GetChannel(id); err == nil {
			return ch
		}
		if c, err := b.Client.GetChat(id); err == nil {
			return c
		}
		return nil
	}
	return tctx
}
