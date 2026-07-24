package botmgr

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/config"
	"github.com/amarnathcjd/tgbotd/internal/storage"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

type Bot struct {
	Token     string
	TokenHash [32]byte
	BotID     int64
	Client    *telegram.Client
	Updates   *UpdateBuffer

	log        *slog.Logger
	store      *storage.Store
	me         *telegram.UserObj
	GetMeCache []byte

	cmdMu    sync.RWMutex
	cmdCache map[string][]byte // key = scope-canon|lang; value = pre-encoded {"ok":true,"result":[...]}
	cmdGen   uint64            // bumped on invalidate; stale Puts are dropped

	// Shared BotInfo cache: one BotsGetBotInfo RPC feeds name/description/short_description.
	// Keyed by lang_code.
	biMu    sync.RWMutex
	biCache map[string]*BotInfoEntry
	biGen   uint64
}

type BotInfoEntry struct {
	Name             string
	Description      string
	ShortDescription string
}

// CommandCacheGet returns a cached getMyCommands response body and the
// generation observed. Callers must pass gen back to CommandCachePut so
// stale writes get dropped after a concurrent invalidate.
func (b *Bot) CommandCacheGet(key string) ([]byte, uint64) {
	b.cmdMu.RLock()
	defer b.cmdMu.RUnlock()
	return b.cmdCache[key], b.cmdGen
}

// CommandCachePut stores a pre-encoded getMyCommands envelope, but only if
// gen matches the current generation. If a concurrent invalidate ran between
// the caller's Get and Put, the Put is silently dropped.
func (b *Bot) CommandCachePut(key string, gen uint64, payload []byte) {
	b.cmdMu.Lock()
	defer b.cmdMu.Unlock()
	if b.cmdGen != gen {
		return
	}
	if b.cmdCache == nil {
		b.cmdCache = make(map[string][]byte, 4)
	}
	b.cmdCache[key] = payload
}

// CommandCacheInvalidate drops every cached getMyCommands entry and bumps
// the generation, so any in-flight Put with an older gen is dropped.
func (b *Bot) CommandCacheInvalidate() {
	b.cmdMu.Lock()
	b.cmdCache = nil
	b.cmdGen++
	b.cmdMu.Unlock()
}

// BotInfoCacheGet returns cached BotInfo for lang and current gen.
func (b *Bot) BotInfoCacheGet(lang string) (*BotInfoEntry, uint64) {
	b.biMu.RLock()
	defer b.biMu.RUnlock()
	return b.biCache[lang], b.biGen
}

// BotInfoCachePut stores BotInfo for lang, guarded by gen.
func (b *Bot) BotInfoCachePut(lang string, gen uint64, e *BotInfoEntry) {
	b.biMu.Lock()
	defer b.biMu.Unlock()
	if b.biGen != gen {
		return
	}
	if b.biCache == nil {
		b.biCache = make(map[string]*BotInfoEntry, 2)
	}
	b.biCache[lang] = e
}

// BotInfoCacheInvalidate wipes cached BotInfo for all langs — one lang's
// mutation could affect fallbacks so we clear everything.
func (b *Bot) BotInfoCacheInvalidate() {
	b.biMu.Lock()
	b.biCache = nil
	b.biGen++
	b.biMu.Unlock()
}

type Manager struct {
	cfg   *config.Config
	store *storage.Store
	log   *slog.Logger

	mu   sync.RWMutex
	bots map[string]*Bot
	init map[string]*sync.Mutex
}

func New(cfg *config.Config, store *storage.Store, log *slog.Logger) *Manager {
	return &Manager{
		cfg:   cfg,
		store: store,
		log:   log,
		bots:  make(map[string]*Bot),
		init:  make(map[string]*sync.Mutex),
	}
}

func validateToken(token string) (int64, error) {
	i := strings.IndexByte(token, ':')
	if i <= 0 || i > 20 {
		return 0, errors.New("bad token format")
	}
	id, err := strconv.ParseInt(token[:i], 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("bad bot id in token")
	}
	if len(token[i+1:]) < 30 {
		return 0, errors.New("bad token secret")
	}
	return id, nil
}

func (m *Manager) Get(ctx context.Context, token string) (*Bot, error) {
	m.mu.RLock()
	b, ok := m.bots[token]
	m.mu.RUnlock()
	if ok {
		return b, nil
	}

	m.mu.Lock()
	if b, ok = m.bots[token]; ok {
		m.mu.Unlock()
		return b, nil
	}
	mu, ok := m.init[token]
	if !ok {
		mu = &sync.Mutex{}
		m.init[token] = mu
	}
	m.mu.Unlock()

	mu.Lock()
	defer mu.Unlock()
	m.mu.RLock()
	if b, ok = m.bots[token]; ok {
		m.mu.RUnlock()
		return b, nil
	}
	m.mu.RUnlock()

	b, err := m.build(ctx, token)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.bots[token] = b
	delete(m.init, token)
	m.mu.Unlock()
	return b, nil
}

func (m *Manager) build(ctx context.Context, token string) (*Bot, error) {
	botID, err := validateToken(token)
	if err != nil {
		return nil, botapi.Errorf(401, "Unauthorized: %s", err.Error())
	}

	sessPath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("bot_%d.session", botID))
	cachePath := filepath.Join(m.cfg.DataDir, fmt.Sprintf("bot_%d.cache", botID))

	cli, err := telegram.NewClient(telegram.ClientConfig{
		AppID:        m.cfg.APIID,
		AppHash:      m.cfg.APIHash,
		Session:      sessPath,
		SessionName:  fmt.Sprintf("bot_%d", botID),
		Cache:        telegram.NewCache(cachePath),
		LogLevel:     telegram.LogWarn,
		TestMode:     m.cfg.TestDC,
		NoUpdates:    true, // We handle updates ourselves via MTProto layer
		FloodHandler: func(err error) bool { return false },
	})
	if err != nil {
		return nil, botapi.Errorf(500, "gogram: NewClient: %s", err.Error())
	}
	if err := cli.Connect(); err != nil {
		return nil, botapi.Errorf(500, "gogram: connect: %s", err.Error())
	}

	authed, _ := cli.IsAuthorized()
	if !authed {
		if err := cli.LoginBot(token); err != nil {
			return nil, mapRPCError(err)
		}
	}

	me := cli.Me()
	if me == nil {
		return nil, botapi.Errorf(500, "gogram: nil Me() after login")
	}

	if err := m.store.UpsertBot(ctx, token, me.ID); err != nil {
		m.log.Warn("upsert bot row failed", "err", err)
	}

	b := &Bot{
		Token:     token,
		TokenHash: sha256.Sum256([]byte(token)),
		BotID:     me.ID,
		Client:    cli,
		Updates:   NewUpdateBuffer(),
		log:       m.log.With("bot_id", me.ID),
		store:     m.store,
		me:        me,
	}
	if enc, err := json.Marshal(tlate.SelfUser(me)); err == nil {
		b.GetMeCache = append([]byte(`{"ok":true,"result":`), enc...)
		b.GetMeCache = append(b.GetMeCache, '}')
	}

	// Tap the RAW MTProto update stream via AddCustomServerRequestHandler.
	// This bypasses gogram's dispatcher entirely (which has PTS/QTS gap
	// tracking that misbehaves under concurrent SendMessage+update reception
	// in some scenarios). We handle unpacking of UpdatesObj / UpdateShort*
	// containers ourselves, so the bot sees every update Telegram sends.
	cli.MTProto.AddCustomServerRequestHandler(func(u any) bool {
		b.dispatchIntercepted(u)
		return false // let gogram's own dispatcher also run (for RPC responses / peer cache updates)
	})

	return b, nil
}

func (b *Bot) Me() *telegram.UserObj { return b.me }

// dispatchIntercepted unpacks MTProto update containers received directly from
// the transport layer (before gogram's Seq/PTS gap detector) and pushes every
// inner Update to the buffer. Bypasses gogram's dispatcher entirely for the
// message-delivery path.
func (b *Bot) dispatchIntercepted(u any) {
	switch upd := u.(type) {
	case *telegram.UpdatesObj:
		if b.Client != nil && b.Client.Cache != nil {
			b.Client.Cache.UpdatePeersToCache(upd.Users, upd.Chats)
		}
		for _, inner := range upd.Updates {
			b.pushInnerUpdate(inner)
		}
	case *telegram.UpdatesCombined:
		if b.Client != nil && b.Client.Cache != nil {
			b.Client.Cache.UpdatePeersToCache(upd.Users, upd.Chats)
		}
		for _, inner := range upd.Updates {
			b.pushInnerUpdate(inner)
		}
	case *telegram.UpdateShort:
		b.pushInnerUpdate(upd.Update)
	case *telegram.UpdateShortMessage:
		msg := &telegram.MessageObj{
			ID: upd.ID, Out: upd.Out, Mentioned: upd.Mentioned, Message: upd.Message,
			MediaUnread: upd.MediaUnread,
			FromID:      &telegram.PeerUser{UserID: upd.UserID},
			PeerID:      &telegram.PeerUser{UserID: upd.UserID},
			Date:        upd.Date, Entities: upd.Entities, FwdFrom: upd.FwdFrom,
			ReplyTo: upd.ReplyTo, ViaBotID: upd.ViaBotID, TtlPeriod: upd.TtlPeriod, Silent: upd.Silent,
		}
		b.pushInnerUpdate(&telegram.UpdateNewMessage{Message: msg, Pts: upd.Pts, PtsCount: upd.PtsCount})
	case *telegram.UpdateShortChatMessage:
		msg := &telegram.MessageObj{
			ID: upd.ID, Out: upd.Out, Mentioned: upd.Mentioned, Message: upd.Message,
			MediaUnread: upd.MediaUnread,
			FromID:      &telegram.PeerUser{UserID: upd.FromID},
			PeerID:      &telegram.PeerChat{ChatID: upd.ChatID},
			Date:        upd.Date, Entities: upd.Entities, FwdFrom: upd.FwdFrom,
			ReplyTo: upd.ReplyTo, ViaBotID: upd.ViaBotID, TtlPeriod: upd.TtlPeriod, Silent: upd.Silent,
		}
		b.pushInnerUpdate(&telegram.UpdateNewMessage{Message: msg, Pts: upd.Pts, PtsCount: upd.PtsCount})
	}
}

func (b *Bot) pushInnerUpdate(u telegram.Update) {
	if u == nil {
		return
	}
	Tracef("INTERCEPT type=%T", u)
	b.Updates.Push(u)
}


func (m *Manager) Store() *storage.Store { return m.store }

// Prewarm loads every known bot from persistent storage and initializes its
// gogram client + MTProto session in background goroutines. Eliminates the
// first-request 5-6s cold start latency by paying that cost at server boot.
func (m *Manager) Prewarm(ctx context.Context) {
	tokens, err := m.store.ListBotTokens(ctx)
	if err != nil {
		m.log.Warn("prewarm: list tokens failed", "err", err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	m.log.Info("prewarming bots", "count", len(tokens))
	var wg sync.WaitGroup
	for _, token := range tokens {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			if _, err := m.Get(ctx, tok); err != nil {
				m.log.Warn("prewarm bot failed", "err", err)
			}
		}(token)
	}
	wg.Wait()
	m.log.Info("prewarm complete", "count", len(tokens))
}

func (m *Manager) ActiveBots() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.bots)
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	bots := make([]*Bot, 0, len(m.bots))
	for _, b := range m.bots {
		bots = append(bots, b)
	}
	m.bots = map[string]*Bot{}
	m.init = map[string]*sync.Mutex{}
	m.mu.Unlock()

	stopAllDeliverers()
	for _, b := range bots {
		if b.Client != nil {
			_ = b.Client.Stop()
		}
	}
}

func mapRPCError(err error) *botapi.APIError {
	if err == nil {
		return nil
	}
	msg := err.Error()
	code := 400
	desc := msg
	// gogram wraps errors as "sending MethodName: [TAG] human text (method:...)".
	// Extract the [TAG] wherever it appears — not just at msg[0].
	tag := ""
	if start := strings.IndexByte(msg, '['); start >= 0 {
		if end := strings.IndexByte(msg[start:], ']'); end > 0 {
			tag = msg[start+1 : start+end]
		}
	}
	if tag != "" {
		switch {
		case tag == "ACCESS_TOKEN_INVALID", tag == "ACCESS_TOKEN_EXPIRED",
			tag == "AUTH_KEY_INVALID", tag == "AUTH_KEY_UNREGISTERED",
			tag == "SESSION_REVOKED", tag == "USER_DEACTIVATED_BAN":
			code = 401
		case strings.HasPrefix(tag, "FLOOD_WAIT_"),
			strings.HasPrefix(tag, "FLOOD_PREMIUM_WAIT_"),
			strings.HasPrefix(tag, "SLOWMODE_WAIT_"):
			code = 429
		case tag == "CHAT_WRITE_FORBIDDEN", tag == "USER_BANNED_IN_CHANNEL",
			tag == "CHANNEL_PRIVATE", tag == "USER_IS_BLOCKED",
			tag == "USER_DEACTIVATED", tag == "CHAT_ADMIN_REQUIRED",
			tag == "RIGHT_FORBIDDEN":
			code = 403
		}
		desc = httpStatusText(code) + ": " + tag
	}
	ae := &botapi.APIError{Code: code, Description: desc}
	if code == 429 {
		if secs := telegram.GetFloodWait(err); secs > 0 {
			ae.Parameters = &botapi.ResponseParameters{RetryAfter: secs}
			ae.Description = fmt.Sprintf("Too Many Requests: retry after %d", secs)
		}
	}
	return ae
}

func MapRPCError(err error) *botapi.APIError { return mapRPCError(err) }

func httpStatusText(code int) string {
	switch code {
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	}
	return "Bad Request"
}
