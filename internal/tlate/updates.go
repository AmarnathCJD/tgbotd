package tlate

import (
	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
)

type TranslateContext struct {
	SelfID     int64
	Users      map[int64]*telegram.UserObj
	Chats      map[int64]telegram.Chat
	UserLookup func(int64) *telegram.UserObj
	ChatLookup func(int64) telegram.Chat
}

func (t *TranslateContext) User(id int64) *telegram.UserObj {
	if u, ok := t.Users[id]; ok {
		return u
	}
	if t.UserLookup != nil {
		if u := t.UserLookup(id); u != nil {
			t.Users[id] = u
			return u
		}
	}
	return nil
}

func (t *TranslateContext) Chat(id int64) telegram.Chat {
	if c, ok := t.Chats[id]; ok && c != nil {
		return c
	}
	if t.ChatLookup != nil {
		if c := t.ChatLookup(id); c != nil {
			t.Chats[id] = c
			return c
		}
	}
	return nil
}

func UpdateToBotAPI(u telegram.Update, ctx *TranslateContext) *botapi.Update {
	if u == nil {
		return nil
	}
	out := &botapi.Update{}
	switch upd := u.(type) {
	case *telegram.UpdateNewMessage:
		if m, ok := upd.Message.(*telegram.MessageObj); ok {
			if m.Out {
				return nil
			}
			if _, isChannel := m.PeerID.(*telegram.PeerChannel); isChannel {
				out.ChannelPost = MessageObjToBotAPICtx(m, ctx)
			} else {
				out.Message = MessageObjToBotAPICtx(m, ctx)
			}
		}
	case *telegram.UpdateNewChannelMessage:
		if m, ok := upd.Message.(*telegram.MessageObj); ok {
			if m.Out {
				return nil
			}
			if isBroadcastChannel(m.PeerID, ctx) {
				out.ChannelPost = MessageObjToBotAPICtx(m, ctx)
			} else {
				out.Message = MessageObjToBotAPICtx(m, ctx)
			}
		}
	case *telegram.UpdateEditMessage:
		if m, ok := upd.Message.(*telegram.MessageObj); ok {
			if m.Out {
				return nil
			}
			out.EditedMessage = MessageObjToBotAPICtx(m, ctx)
		}
	case *telegram.UpdateEditChannelMessage:
		if m, ok := upd.Message.(*telegram.MessageObj); ok {
			if m.Out {
				return nil
			}
			if isBroadcastChannel(m.PeerID, ctx) {
				out.EditedChannelPost = MessageObjToBotAPICtx(m, ctx)
			} else {
				out.EditedMessage = MessageObjToBotAPICtx(m, ctx)
			}
		}
	case *telegram.UpdateBotCallbackQuery:
		out.CallbackQuery = &botapi.CallbackQuery{
			ID:           itoa64(upd.QueryID),
			ChatInstance: itoa64(upd.ChatInstance),
			Data:         string(upd.Data),
		}
		if u, ok := ctx.Users[upd.UserID]; ok {
			out.CallbackQuery.From = *UserFromObj(u)
		} else {
			out.CallbackQuery.From = botapi.User{ID: upd.UserID}
		}
		if upd.GameShortName != "" {
			out.CallbackQuery.GameShortName = upd.GameShortName
		}
	case *telegram.UpdateInlineBotCallbackQuery:
		out.CallbackQuery = &botapi.CallbackQuery{
			ID:              itoa64(upd.QueryID),
			ChatInstance:    itoa64(upd.ChatInstance),
			Data:            string(upd.Data),
			InlineMessageID: inlineMsgID(upd.MsgID),
		}
		if u, ok := ctx.Users[upd.UserID]; ok {
			out.CallbackQuery.From = *UserFromObj(u)
		} else {
			out.CallbackQuery.From = botapi.User{ID: upd.UserID}
		}
	case *telegram.UpdateBotInlineQuery:
		out.InlineQuery = &botapi.InlineQuery{
			ID:     itoa64(upd.QueryID),
			Query:  upd.Query,
			Offset: upd.Offset,
		}
		if u, ok := ctx.Users[upd.UserID]; ok {
			out.InlineQuery.From = *UserFromObj(u)
		} else {
			out.InlineQuery.From = botapi.User{ID: upd.UserID}
		}
		if upd.Geo != nil {
			if pt, ok := upd.Geo.(*telegram.GeoPointObj); ok {
				out.InlineQuery.Location = &botapi.Location{Latitude: pt.Lat, Longitude: pt.Long}
			}
		}
		switch upd.PeerType {
		case telegram.InlineQueryPeerTypeSameBotPm, telegram.InlineQueryPeerTypePm:
			out.InlineQuery.ChatType = "private"
		case telegram.InlineQueryPeerTypeChat:
			out.InlineQuery.ChatType = "group"
		case telegram.InlineQueryPeerTypeMegagroup:
			out.InlineQuery.ChatType = "supergroup"
		case telegram.InlineQueryPeerTypeBroadcast:
			out.InlineQuery.ChatType = "channel"
		}
	case *telegram.UpdateBotInlineSend:
		envelope := map[string]any{
			"result_id": upd.ID,
			"query":     upd.Query,
		}
		if u, ok := ctx.Users[upd.UserID]; ok {
			envelope["from"] = UserFromObj(u)
		} else {
			envelope["from"] = botapi.User{ID: upd.UserID}
		}
		if upd.MsgID != nil {
			envelope["inline_message_id"] = inlineMsgID(upd.MsgID)
		}
		if upd.Geo != nil {
			if pt, ok := upd.Geo.(*telegram.GeoPointObj); ok {
				envelope["location"] = map[string]float64{"latitude": pt.Lat, "longitude": pt.Long}
			}
		}
		b, _ := jsonMarshal(envelope)
		out.ChosenInlineResult = b
	case *telegram.UpdateMessagePoll:
		if upd.Poll != nil {
			out.Poll = pollToBotAPIExport(upd.Poll, upd.Results)
		}
	case *telegram.UpdateMessagePollVote:
		envelope := map[string]any{
			"poll_id":    itoa64(upd.PollID),
			"option_ids": optionsToIndexes(upd.Options),
		}
		id := peerToUserID(upd.Peer)
		if u := ctx.User(id); u != nil {
			envelope["user"] = UserFromObj(u)
		} else {
			envelope["user"] = botapi.User{ID: id}
		}
		b, _ := jsonMarshal(envelope)
		out.PollAnswer = b
	case *telegram.UpdateChannelParticipant:
		if b := buildChatMemberUpdated(upd.Date, upd.ActorID, upd.UserID,
			upd.PrevParticipant, upd.NewParticipant, upd.Invite, ctx,
			int64(upd.ChannelID), true); b != nil {
			out.ChatMember = b
		}
	case *telegram.UpdateChatParticipant:
		if b := buildChatMemberUpdated(upd.Date, upd.ActorID, upd.UserID,
			upd.PrevParticipant, upd.NewParticipant, upd.Invite, ctx,
			int64(upd.ChatID), false); b != nil {
			out.ChatMember = b
		}
	case *telegram.UpdateBotChatInviteRequester:
		envelope := map[string]any{
			"chat":         ChatFromPeerCtx(upd.Peer, ctx),
			"user_chat_id": peerToUserID(upd.Peer),
			"date":         upd.Date,
			"bio":          upd.About,
		}
		if u, ok := ctx.Users[upd.UserID]; ok {
			envelope["from"] = UserFromObj(u)
		} else {
			envelope["from"] = botapi.User{ID: upd.UserID}
		}
		if upd.Invite != nil {
			if inv, ok := upd.Invite.(*telegram.ChatInviteExported); ok {
				envelope["invite_link"] = map[string]any{
					"invite_link": inv.Link,
					"creator":     UserFromObj(ctx.Users[inv.AdminID]),
					"is_primary":  inv.Permanent,
					"is_revoked":  inv.Revoked,
				}
			}
		}
		b, _ := jsonMarshal(envelope)
		out.ChatJoinRequest = b
	case *telegram.UpdateBotMessageReaction:
		envelope := map[string]any{
			"chat":         ChatFromPeerCtx(upd.Peer, ctx),
			"message_id":   upd.MsgID,
			"date":         upd.Date,
			"old_reaction": reactionsToBotAPI(upd.OldReactions),
			"new_reaction": reactionsToBotAPI(upd.NewReactions),
		}
		if upd.Actor != nil {
			switch actor := upd.Actor.(type) {
			case *telegram.PeerUser:
				if u := ctx.User(actor.UserID); u != nil {
					envelope["user"] = UserFromObj(u)
				} else {
					envelope["user"] = botapi.User{ID: actor.UserID}
				}
			case *telegram.PeerChannel, *telegram.PeerChat:
				envelope["actor_chat"] = ChatFromPeerCtx(upd.Actor, ctx)
			}
		}
		b, _ := jsonMarshal(envelope)
		out.MessageReaction = b
	case *telegram.UpdateBotMessageReactions:
		envelope := map[string]any{
			"chat":       ChatFromPeerCtx(upd.Peer, ctx),
			"message_id": upd.MsgID,
			"date":       upd.Date,
			"reactions":  reactionCountsToBotAPI(upd.Reactions),
		}
		b, _ := jsonMarshal(envelope)
		out.MessageReactionCount = b
	case *telegram.UpdateBotChatBoost:
		envelope := map[string]any{
			"chat": ChatFromPeerCtx(upd.Peer, ctx),
			"boost": map[string]any{
				"add_date":        boostAddDate(upd.Boost),
				"expiration_date": boostExpireDate(upd.Boost),
				"boost_id":        boostID(upd.Boost),
			},
		}
		b, _ := jsonMarshal(envelope)
		out.ChatBoost = b
	case *telegram.UpdateBotBusinessConnect:
		if upd.Connection != nil {
			envelope := map[string]any{
				"id":           upd.Connection.ConnectionID,
				"user":         botapi.User{ID: upd.Connection.UserID},
				"user_chat_id": upd.Connection.UserID,
				"date":         upd.Connection.Date,
				"is_enabled":   !upd.Connection.Disabled,
			}
			b, _ := jsonMarshal(envelope)
			out.BusinessConnection = b
		}
	case *telegram.UpdateBotNewBusinessMessage:
		if m, ok := upd.Message.(*telegram.MessageObj); ok {
			out.BusinessMessage = MessageObjToBotAPICtx(m, ctx)
		}
	case *telegram.UpdateBotEditBusinessMessage:
		if m, ok := upd.Message.(*telegram.MessageObj); ok {
			out.EditedBusinessMessage = MessageObjToBotAPICtx(m, ctx)
		}
	case *telegram.UpdateBotDeleteBusinessMessage:
		envelope := map[string]any{
			"business_connection_id": upd.ConnectionID,
			"chat":                   ChatFromPeerCtx(upd.Peer, ctx),
			"message_ids":            upd.Messages,
		}
		b, _ := jsonMarshal(envelope)
		out.DeletedBusinessMessages = b
	case *telegram.UpdateBotPurchasedPaidMedia:
		envelope := map[string]any{
			"from":               botapi.User{ID: upd.UserID},
			"paid_media_payload": upd.Payload,
		}
		b, _ := jsonMarshal(envelope)
		out.PurchasedPaidMedia = b
	default:
		return nil
	}

	if isEmpty(out) {
		return nil
	}
	return out
}

func isEmpty(u *botapi.Update) bool {
	return u.Message == nil && u.EditedMessage == nil && u.ChannelPost == nil &&
		u.EditedChannelPost == nil && u.CallbackQuery == nil && u.InlineQuery == nil &&
		u.Poll == nil && u.BusinessMessage == nil && u.EditedBusinessMessage == nil &&
		u.GuestMessage == nil && len(u.ChosenInlineResult) == 0 && len(u.PollAnswer) == 0 &&
		len(u.ChatMember) == 0 && len(u.ChatJoinRequest) == 0 && len(u.MessageReaction) == 0 &&
		len(u.MessageReactionCount) == 0 && len(u.ChatBoost) == 0 &&
		len(u.BusinessConnection) == 0 && len(u.DeletedBusinessMessages) == 0 &&
		len(u.PurchasedPaidMedia) == 0
}

func isBroadcastChannel(p telegram.Peer, ctx *TranslateContext) bool {
	pc, ok := p.(*telegram.PeerChannel)
	if !ok {
		return false
	}
	if ch, ok := ctx.Chat(pc.ChannelID).(*telegram.Channel); ok {
		return ch.Broadcast
	}
	return false
}

func inlineMsgID(id telegram.InputBotInlineMessageID) string {
	if id == nil {
		return ""
	}
	switch v := id.(type) {
	case *telegram.InputBotInlineMessageIDObj:
		return itoa64(int64(v.DcID)) + ":" + itoa64(v.ID) + ":" + itoa64(v.AccessHash)
	case *telegram.InputBotInlineMessageID64:
		return itoa64(int64(v.DcID)) + ":" + itoa64(v.OwnerID) + ":" + itoa64(int64(v.ID)) + ":" + itoa64(v.AccessHash)
	}
	return ""
}
