package server

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

var guTrace = os.Getenv("TGBOTD_TRACE") == "1"

func traceGU(format string, args ...any) {
	if guTrace {
		botmgr.Tracef("[gu] "+format, args...)
	}
}

func getUpdates(s *Server, r *Request) (any, error) {
	limit := 100
	if n, ok := paramInt64(r, "limit"); ok && n > 0 {
		if n > 100 {
			n = 100
		}
		limit = int(n)
	}
	timeoutSec := int64(0)
	if n, ok := paramInt64(r, "timeout"); ok && n > 0 {
		if n > 50 {
			n = 50
		}
		timeoutSec = n
	}
	offset := int64(0)
	if n, ok := paramInt64(r, "offset"); ok && n > 0 {
		offset = n
	}

	release := r.Bot.Updates.TakeLock()
	defer release()

	if offset > 0 {
		r.Bot.Updates.Ack(offset)
	}

	items := r.Bot.Updates.Wait(r.Ctx, offset, limit, time.Duration(timeoutSec)*time.Second)
	traceGU("getUpdates offset=%d timeout=%d items_in_snapshot=%d", offset, timeoutSec, len(items))
	if len(items) == 0 {
		return []*botapi.Update{}, nil
	}

	tctx := r.Bot.BuildTranslateContext()
	out := make([]*botapi.Update, 0, len(items))
	for _, item := range items {
		bu := tlate.UpdateToBotAPI(item.Update, tctx)
		if bu == nil {
			traceGU("  id=%d TRANSLATOR RETURNED NIL", item.ID)
			continue
		}
		bu.UpdateID = item.ID
		out = append(out, bu)
		traceGU("  id=%d DELIVERED", item.ID)
	}
	// Bot API convention: getUpdates without an explicit offset does NOT
	// consume; offset in the next call is what confirms delivery. But our
	// buffer keeps untranslatable items forever if the client only advances
	// offset for translatable ones. Auto-ack items we've consumed here
	// whose translator returned nil, so the buffer drains untranslatables
	// even if the client doesn't see them. The client's own offset still
	// works for translatables.
	lastID := items[len(items)-1].ID
	if len(out) == 0 {
		// All items were untranslatable — drain them.
		r.Bot.Updates.Ack(lastID + 1)
	} else {
		// Ack any untranslatable items that came BEFORE the last delivered one.
		lastDeliveredID := out[len(out)-1].UpdateID
		if lastID > lastDeliveredID {
			// The tail items after the last delivered one all returned nil;
			// drop them so we don't keep re-emitting them.
			r.Bot.Updates.Ack(lastID + 1)
		}
	}
	return out, nil
}

func setWebhook(s *Server, r *Request) (any, error) {
	url, _ := paramString(r, "url")
	if url == "" {
		return nil, botapi.ErrBadRequest("field \"url\" is required")
	}
	secret, _ := paramString(r, "secret_token")
	ip, _ := paramString(r, "ip_address")
	maxConns := int64(40)
	if v, ok := paramInt64(r, "max_connections"); ok {
		maxConns = v
	}
	dropPending, _ := paramBool(r, "drop_pending_updates")

	var allowed []string
	if raw, ok := paramRaw(r, "allowed_updates"); ok {
		_ = json.Unmarshal(raw, &allowed)
	}
	allowedJSON, _ := json.Marshal(allowed)

	if dropPending {
		r.Bot.Updates.Clear()
	}
	if err := s.mgr.Store().SetWebhook(context.Background(), r.Bot.Token, url, secret, ip, int(maxConns), string(allowedJSON), dropPending); err != nil {
		return nil, botapi.ErrInternal(err.Error())
	}
	r.Bot.StartWebhook(botmgr.WebhookConfig{
		URL:         url,
		Secret:      secret,
		IP:          ip,
		MaxConns:    int(maxConns),
		Allowed:     allowed,
		DropPending: dropPending,
	})
	return true, nil
}

func deleteWebhook(s *Server, r *Request) (any, error) {
	dropPending, _ := paramBool(r, "drop_pending_updates")
	if err := s.mgr.Store().SetWebhook(context.Background(), r.Bot.Token, "", "", "", 40, "", dropPending); err != nil {
		return nil, botapi.ErrInternal(err.Error())
	}
	if dropPending {
		r.Bot.Updates.Clear()
	}
	r.Bot.StartWebhook(botmgr.WebhookConfig{})
	return true, nil
}

func getWebhookInfo(s *Server, r *Request) (any, error) {
	row, err := s.mgr.Store().GetBot(context.Background(), r.Bot.Token)
	if err != nil {
		return nil, botapi.ErrInternal(err.Error())
	}
	info := &botapi.WebhookInfo{
		PendingUpdateCount: r.Bot.Updates.Len(),
	}
	if row != nil {
		info.URL = row.WebhookURL
		info.IPAddress = row.WebhookIP
		info.MaxConnections = row.MaxConns
		if row.AllowedUpdates != "" {
			_ = json.Unmarshal([]byte(row.AllowedUpdates), &info.AllowedUpdates)
		}
	}
	return info, nil
}
