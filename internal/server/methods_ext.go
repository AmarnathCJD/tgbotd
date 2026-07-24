package server

import (
	"encoding/json"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

// Extension methods — features MTProto exposes but the upstream Bot API
// deliberately does not. Everything here is opt-in; nothing above these
// registrations replaces or shadows an official method.

func init() {
	register("resolveusername", resolveUsername)
	register("getmessages", getMessagesByID)
}

// ResolveUsernameResult mirrors the "resolved peer" data Bot API leaves out.
type ResolveUsernameResult struct {
	// One of user / chat / channel — the resolved kind.
	Type string `json:"type"`
	// Bot API-style chat_id (positive user id, negative chat id, -100... channel id).
	ChatID int64 `json:"chat_id"`
	// Full user/chat/channel echo when we can build one from the peer cache.
	User *botapi.User `json:"user,omitempty"`
	Chat *botapi.Chat `json:"chat,omitempty"`
	// AccessHash — the raw MTProto access hash. Rarely useful to end users
	// but exposed for advanced clients that want to construct InputPeer.
	AccessHash int64 `json:"access_hash,omitempty"`
}

// resolveUsername → gogram ResolveUsername. Accepts either "@handle",
// "handle", or a "t.me/handle" URL. Returns a Bot API-style chat_id plus
// the resolved kind and whatever user/chat metadata Telegram sends back.
func resolveUsername(s *Server, r *Request) (any, error) {
	uname, err := requireString(r, "username")
	if err != nil {
		return nil, err
	}
	resolved, err := r.Bot.Client.ResolveUsername(uname)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := &ResolveUsernameResult{}
	switch v := resolved.(type) {
	case *telegram.UserObj:
		out.Type = "user"
		out.ChatID = v.ID
		out.AccessHash = v.AccessHash
		out.User = tlate.UserFromObj(v)
	case *telegram.ChatObj:
		out.Type = "group"
		out.ChatID = -v.ID
		out.Chat = &botapi.Chat{ID: -v.ID, Type: "group", Title: v.Title}
	case *telegram.Channel:
		out.ChatID = -1_000_000_000_000 - v.ID
		out.AccessHash = v.AccessHash
		if v.Broadcast {
			out.Type = "channel"
		} else {
			out.Type = "supergroup"
		}
		out.Chat = &botapi.Chat{
			ID:       out.ChatID,
			Type:     out.Type,
			Title:    v.Title,
			Username: v.Username,
			IsForum:  v.Forum,
		}
	default:
		return nil, botapi.Errorf(400, "unknown resolved peer type")
	}
	return out, nil
}

// getMessages by id — messages.getMessages / channels.getMessages depending on peer.
// Not in Bot API. Params: chat_id + message_ids ([]int).
func getMessagesByID(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "message_ids")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"message_ids\" is required")
	}
	var ids []int32
	if err := jsonUnmarshalInts(raw, &ids); err != nil {
		return nil, botapi.ErrBadRequest("message_ids must be an array of integers")
	}
	if len(ids) == 0 {
		return []any{}, nil
	}
	// gogram's SearchOption with IDs threaded through GetMessages.
	msgs, err := r.Bot.Client.GetMessages(peer, &telegram.SearchOption{IDs: intsToAny(ids)})
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	tctx := r.Bot.BuildTranslateContext()
	out := make([]*botapi.Message, 0, len(msgs))
	for i := range msgs {
		nm := msgs[i]
		bm := tlate.MessageObjToBotAPICtx(newMessageToObj(&nm), tctx)
		if bm != nil {
			out = append(out, bm)
		}
	}
	return out, nil
}

func intsToAny(ids []int32) []int32 {
	// gogram accepts []int32 directly in SearchOption.IDs (typed as any []int).
	return ids
}

// jsonUnmarshalInts decodes a JSON array of ints (accepting numeric strings too).
func jsonUnmarshalInts(raw []byte, out *[]int32) error {
	var nums []int64
	if err := json.Unmarshal(raw, &nums); err == nil {
		result := make([]int32, len(nums))
		for i, n := range nums {
			result[i] = int32(n)
		}
		*out = result
		return nil
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		result := make([]int32, 0, len(strs))
		for _, s := range strs {
			var n int64
			if _, err := jsonParseInt(s, &n); err == nil {
				result = append(result, int32(n))
			}
		}
		*out = result
		return nil
	}
	return botapi.ErrBadRequest("not a JSON array")
}

// extractUsersChatsFromMany scans a batch of NewMessage results and returns
// aggregated users/chats maps for translator context.
func extractUsersChatsFromMany(msgs []telegram.NewMessage) (map[int64]*telegram.UserObj, map[int64]telegram.Chat) {
	users := map[int64]*telegram.UserObj{}
	chats := map[int64]telegram.Chat{}
	for i := range msgs {
		nm := &msgs[i]
		if nm.Sender != nil {
			users[nm.Sender.ID] = nm.Sender
		}
		if nm.Channel != nil {
			chats[nm.Channel.ID] = nm.Channel
		}
	}
	return users, chats
}
