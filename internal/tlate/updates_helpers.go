package tlate

import (
	"encoding/json"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
)

func jsonMarshal(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func pollToBotAPIExport(p *telegram.Poll, r *telegram.PollResults) *botapi.Poll {
	return pollToBotAPI(p, r)
}

func optionsToIndexes(options [][]byte) []int {
	out := make([]int, 0, len(options))
	for i := range options {
		out = append(out, i)
	}
	return out
}

func peerToUserID(p telegram.Peer) int64 {
	switch v := p.(type) {
	case *telegram.PeerUser:
		return v.UserID
	case *telegram.PeerChat:
		return -v.ChatID
	case *telegram.PeerChannel:
		return -1_000_000_000_000 - v.ChannelID
	}
	return 0
}

func buildChatMemberUpdated(
	date int32, actorID, userID int64,
	prev, next any,
	invite telegram.ExportedChatInvite,
	ctx *TranslateContext,
	rawChatID int64, isChannel bool,
) json.RawMessage {
	var chatID int64
	if isChannel {
		chatID = -1_000_000_000_000 - rawChatID
	} else {
		chatID = -rawChatID
	}
	chat := botapi.Chat{ID: chatID}
	if isChannel {
		if ch, ok := ctx.Chats[rawChatID].(*telegram.Channel); ok {
			chat.Type = "supergroup"
			if ch.Broadcast {
				chat.Type = "channel"
			}
			chat.Title = ch.Title
			chat.Username = ch.Username
			chat.IsForum = ch.Forum
		}
	} else {
		chat.Type = "group"
		if ch, ok := ctx.Chats[rawChatID].(*telegram.ChatObj); ok {
			chat.Title = ch.Title
		}
	}
	envelope := map[string]any{
		"chat": chat,
		"date": date,
	}
	if u, ok := ctx.Users[actorID]; ok {
		envelope["from"] = UserFromObj(u)
	} else {
		envelope["from"] = botapi.User{ID: actorID}
	}
	envelope["old_chat_member"] = participantToChatMember(prev, userID, ctx)
	envelope["new_chat_member"] = participantToChatMember(next, userID, ctx)
	if invite != nil {
		if inv, ok := invite.(*telegram.ChatInviteExported); ok {
			envelope["invite_link"] = map[string]any{
				"invite_link": inv.Link,
				"is_primary":  inv.Permanent,
				"is_revoked":  inv.Revoked,
			}
		}
	}
	b, _ := jsonMarshal(envelope)
	return b
}

func participantToChatMember(p any, userID int64, ctx *TranslateContext) map[string]any {
	m := map[string]any{"status": "member"}
	if u, ok := ctx.Users[userID]; ok {
		m["user"] = UserFromObj(u)
	} else {
		m["user"] = botapi.User{ID: userID}
	}
	if p == nil {
		return m
	}
	switch v := p.(type) {
	case *telegram.ChannelParticipantObj, *telegram.ChannelParticipantSelf,
		*telegram.ChatParticipantObj:
		m["status"] = "member"
		_ = v
	case *telegram.ChannelParticipantAdmin:
		m["status"] = "administrator"
		if v.AdminRights != nil {
			m["can_be_edited"] = v.CanEdit
			applyAdminRightsToMap(v.AdminRights, m)
		}
		if v.Rank != "" {
			m["custom_title"] = v.Rank
		}
	case *telegram.ChannelParticipantCreator:
		m["status"] = "creator"
		if v.Rank != "" {
			m["custom_title"] = v.Rank
		}
		if v.AdminRights != nil {
			m["is_anonymous"] = v.AdminRights.Anonymous
		}
	case *telegram.ChannelParticipantBanned:
		if v.Left {
			m["status"] = "left"
		} else {
			m["status"] = "restricted"
		}
		if v.BannedRights != nil {
			m["until_date"] = v.BannedRights.UntilDate
			m["can_send_messages"] = !v.BannedRights.SendMessages
		}
	case *telegram.ChannelParticipantLeft:
		m["status"] = "left"
	case *telegram.ChatParticipantAdmin:
		m["status"] = "administrator"
	case *telegram.ChatParticipantCreator:
		m["status"] = "creator"
	}
	return m
}

func applyAdminRightsToMap(r *telegram.ChatAdminRights, m map[string]any) {
	m["is_anonymous"] = r.Anonymous
	m["can_manage_chat"] = r.ChangeInfo
	m["can_delete_messages"] = r.DeleteMessages
	m["can_manage_video_chats"] = r.ManageCall
	m["can_restrict_members"] = r.BanUsers
	m["can_promote_members"] = r.AddAdmins
	m["can_change_info"] = r.ChangeInfo
	m["can_invite_users"] = r.InviteUsers
	m["can_post_messages"] = r.PostMessages
	m["can_edit_messages"] = r.EditMessages
	m["can_pin_messages"] = r.PinMessages
	m["can_manage_topics"] = r.ManageTopics
}

func reactionsToBotAPI(list []telegram.Reaction) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, r := range list {
		switch v := r.(type) {
		case *telegram.ReactionEmoji:
			out = append(out, map[string]any{"type": "emoji", "emoji": v.Emoticon})
		case *telegram.ReactionCustomEmoji:
			out = append(out, map[string]any{"type": "custom_emoji", "custom_emoji_id": itoa64(v.DocumentID)})
		case *telegram.ReactionPaid:
			out = append(out, map[string]any{"type": "paid"})
		}
	}
	return out
}

func boostID(b *telegram.Boost) string {
	if b == nil {
		return ""
	}
	return b.ID
}
func boostAddDate(b *telegram.Boost) int32 {
	if b == nil {
		return 0
	}
	return b.Date
}
func boostExpireDate(b *telegram.Boost) int32 {
	if b == nil {
		return 0
	}
	return b.Expires
}

func reactionCountsToBotAPI(list []*telegram.ReactionCount) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, rc := range list {
		if rc == nil {
			continue
		}
		typeMap := map[string]any{}
		switch v := rc.Reaction.(type) {
		case *telegram.ReactionEmoji:
			typeMap = map[string]any{"type": "emoji", "emoji": v.Emoticon}
		case *telegram.ReactionCustomEmoji:
			typeMap = map[string]any{"type": "custom_emoji", "custom_emoji_id": itoa64(v.DocumentID)}
		case *telegram.ReactionPaid:
			typeMap = map[string]any{"type": "paid"}
		}
		out = append(out, map[string]any{
			"type":        typeMap,
			"total_count": rc.Count,
		})
	}
	return out
}
