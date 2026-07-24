package tlate

import (
	"encoding/json"
	"strconv"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
)

func BotAPIChatID(peerID int64, isChannel bool) int64 {
	if isChannel {
		return -1_000_000_000_000 - peerID
	}
	return peerID
}

func UserFromObj(u *telegram.UserObj) *botapi.User {
	if u == nil {
		return nil
	}
	return &botapi.User{
		ID:                    u.ID,
		IsBot:                 u.Bot,
		FirstName:             u.FirstName,
		LastName:              u.LastName,
		Username:              u.Username,
		LanguageCode:          u.LangCode,
		IsPremium:             u.Premium,
		AddedToAttachmentMenu: u.AttachMenuEnabled,
	}
}

func SelfUser(u *telegram.UserObj) *botapi.User {
	bu := UserFromObj(u)
	if bu == nil {
		return nil
	}
	bu.CanJoinGroups = !u.BotNochats
	bu.CanReadAllGroupMessages = u.BotChatHistory
	bu.SupportsInlineQueries = u.BotInlinePlaceholder != ""
	bu.HasMainWebApp = u.BotHasMainApp
	bu.CanConnectToBusiness = u.BotBusiness
	return bu
}

func PeerToBotAPIChatID(p telegram.Peer) int64 {
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

func ChatFromPeer(p telegram.Peer, users map[int64]*telegram.UserObj, chats map[int64]telegram.Chat) botapi.Chat {
	return ChatFromPeerCtx(p, &TranslateContext{Users: users, Chats: chats})
}

func ChatFromPeerCtx(p telegram.Peer, tctx *TranslateContext) botapi.Chat {
	switch v := p.(type) {
	case *telegram.PeerUser:
		u := tctx.User(v.UserID)
		c := botapi.Chat{ID: v.UserID, Type: "private"}
		if u != nil {
			c.FirstName, c.LastName, c.Username = u.FirstName, u.LastName, u.Username
		}
		return c
	case *telegram.PeerChat:
		c := botapi.Chat{ID: -v.ChatID, Type: "group"}
		if ch, ok := tctx.Chat(v.ChatID).(*telegram.ChatObj); ok {
			c.Title = ch.Title
		}
		return c
	case *telegram.PeerChannel:
		c := botapi.Chat{ID: -1_000_000_000_000 - v.ChannelID, Type: "supergroup"}
		if ch, ok := tctx.Chat(v.ChannelID).(*telegram.Channel); ok {
			c.Title = ch.Title
			c.Username = ch.Username
			c.IsForum = ch.Forum
			if ch.Broadcast {
				c.Type = "channel"
			}
		}
		return c
	}
	return botapi.Chat{}
}

func MessageObjToBotAPI(m *telegram.MessageObj, users map[int64]*telegram.UserObj, chats map[int64]telegram.Chat) *botapi.Message {
	return MessageObjToBotAPICtx(m, &TranslateContext{Users: users, Chats: chats})
}

func MessageObjToBotAPICtx(m *telegram.MessageObj, tctx *TranslateContext) *botapi.Message {
	if m == nil {
		return nil
	}
	bm := &botapi.Message{
		MessageID: int64(m.ID),
		Date:      int64(m.Date),
		Chat:      ChatFromPeerCtx(m.PeerID, tctx),
		Text:      m.Message,
	}
	// Determine From: for outgoing messages in private chats, sender is the bot itself.
	if m.Out {
		if tctx.SelfID != 0 {
			if u := tctx.User(tctx.SelfID); u != nil {
				bm.From = UserFromObj(u)
			} else {
				bm.From = &botapi.User{ID: tctx.SelfID, IsBot: true}
			}
		}
	} else if m.FromID != nil {
		if pu, ok := m.FromID.(*telegram.PeerUser); ok {
			if u := tctx.User(pu.UserID); u != nil {
				bm.From = UserFromObj(u)
			} else {
				bm.From = &botapi.User{ID: pu.UserID}
			}
		}
	} else if pu, isPrivate := m.PeerID.(*telegram.PeerUser); isPrivate {
		if u := tctx.User(pu.UserID); u != nil {
			bm.From = UserFromObj(u)
		} else {
			bm.From = &botapi.User{ID: pu.UserID}
		}
	}
	if m.EditDate != 0 {
		bm.EditDate = int64(m.EditDate)
	}
	if m.Entities != nil {
		bm.Entities = EntitiesToBotAPI(m.Entities)
	}
	if m.Media != nil {
		FillMedia(bm, m.Media)
	}
	if m.ReplyMarkup != nil {
		if b := ReplyMarkupToBotAPI(m.ReplyMarkup); b != nil {
			bm.ReplyMarkup = b
		}
	}
	return bm
}

// ReplyMarkupToBotAPI translates a gogram ReplyMarkup interface into the
// Bot API JSON shape (inline_keyboard / keyboard / remove_keyboard / force_reply).
func ReplyMarkupToBotAPI(rm telegram.ReplyMarkup) json.RawMessage {
	switch v := rm.(type) {
	case *telegram.ReplyInlineMarkup:
		rows := make([][]map[string]any, 0, len(v.Rows))
		for _, row := range v.Rows {
			buttons := make([]map[string]any, 0, len(row.Buttons))
			for _, btn := range row.Buttons {
				buttons = append(buttons, inlineButtonToBotAPI(btn))
			}
			rows = append(rows, buttons)
		}
		b, _ := json.Marshal(map[string]any{"inline_keyboard": rows})
		return b
	case *telegram.ReplyKeyboardMarkup:
		rows := make([][]map[string]any, 0, len(v.Rows))
		for _, row := range v.Rows {
			buttons := make([]map[string]any, 0, len(row.Buttons))
			for _, btn := range row.Buttons {
				buttons = append(buttons, replyButtonToBotAPI(btn))
			}
			rows = append(rows, buttons)
		}
		b, _ := json.Marshal(map[string]any{
			"keyboard":                 rows,
			"resize_keyboard":          v.Resize,
			"one_time_keyboard":        v.SingleUse,
			"selective":                v.Selective,
			"input_field_placeholder":  v.Placeholder,
		})
		return b
	case *telegram.ReplyKeyboardHide:
		b, _ := json.Marshal(map[string]any{"remove_keyboard": true, "selective": v.Selective})
		return b
	case *telegram.ReplyKeyboardForceReply:
		b, _ := json.Marshal(map[string]any{
			"force_reply":              true,
			"selective":                v.Selective,
			"input_field_placeholder":  v.Placeholder,
		})
		return b
	}
	return nil
}

func inlineButtonToBotAPI(b telegram.KeyboardButton) map[string]any {
	switch v := b.(type) {
	case *telegram.KeyboardButtonURL:
		return map[string]any{"text": v.Text, "url": v.URL}
	case *telegram.KeyboardButtonCallback:
		return map[string]any{"text": v.Text, "callback_data": string(v.Data)}
	case *telegram.KeyboardButtonSimpleWebView:
		return map[string]any{"text": v.Text, "web_app": map[string]string{"url": v.URL}}
	case *telegram.KeyboardButtonWebView:
		return map[string]any{"text": v.Text, "web_app": map[string]string{"url": v.URL}}
	case *telegram.KeyboardButtonSwitchInline:
		if v.SamePeer {
			return map[string]any{"text": v.Text, "switch_inline_query_current_chat": v.Query}
		}
		return map[string]any{"text": v.Text, "switch_inline_query": v.Query}
	case *telegram.KeyboardButtonBuy:
		return map[string]any{"text": v.Text, "pay": true}
	case *telegram.KeyboardButtonGame:
		return map[string]any{"text": v.Text, "callback_game": map[string]any{}}
	case *telegram.KeyboardButtonCopy:
		return map[string]any{"text": v.Text, "copy_text": map[string]string{"text": v.CopyText}}
	case *telegram.KeyboardButtonURLAuth:
		return map[string]any{"text": v.Text, "login_url": map[string]any{"url": v.URL, "forward_text": v.FwdText, "bot_username": v.ButtonID}}
	}
	if b == nil {
		return map[string]any{}
	}
	return map[string]any{"text": buttonText(b)}
}

func replyButtonToBotAPI(b telegram.KeyboardButton) map[string]any {
	switch v := b.(type) {
	case *telegram.KeyboardButtonObj:
		return map[string]any{"text": v.Text}
	case *telegram.KeyboardButtonRequestPhone:
		return map[string]any{"text": v.Text, "request_contact": true}
	case *telegram.KeyboardButtonRequestGeoLocation:
		return map[string]any{"text": v.Text, "request_location": true}
	case *telegram.KeyboardButtonRequestPoll:
		return map[string]any{"text": v.Text, "request_poll": map[string]string{}}
	case *telegram.KeyboardButtonSimpleWebView:
		return map[string]any{"text": v.Text, "web_app": map[string]string{"url": v.URL}}
	}
	return map[string]any{"text": buttonText(b)}
}

func buttonText(b telegram.KeyboardButton) string {
	type textHolder interface{ GetText() string }
	if v, ok := b.(interface{ GetText() string }); ok {
		return v.GetText()
	}
	_ = textHolder(nil)
	return ""
}

func EntitiesToBotAPI(list []telegram.MessageEntity) []botapi.MessageEntity {
	out := make([]botapi.MessageEntity, 0, len(list))
	for _, e := range list {
		me := botapi.MessageEntity{}
		switch v := e.(type) {
		case *telegram.MessageEntityMention:
			me.Type, me.Offset, me.Length = "mention", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityHashtag:
			me.Type, me.Offset, me.Length = "hashtag", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityCashtag:
			me.Type, me.Offset, me.Length = "cashtag", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityBotCommand:
			me.Type, me.Offset, me.Length = "bot_command", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityURL:
			me.Type, me.Offset, me.Length = "url", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityEmail:
			me.Type, me.Offset, me.Length = "email", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityBold:
			me.Type, me.Offset, me.Length = "bold", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityItalic:
			me.Type, me.Offset, me.Length = "italic", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityCode:
			me.Type, me.Offset, me.Length = "code", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityPre:
			me.Type, me.Offset, me.Length = "pre", int(v.Offset), int(v.Length)
			me.Language = v.Language
		case *telegram.MessageEntityTextURL:
			me.Type, me.Offset, me.Length = "text_link", int(v.Offset), int(v.Length)
			me.URL = v.URL
		case *telegram.MessageEntityMentionName:
			me.Type, me.Offset, me.Length = "text_mention", int(v.Offset), int(v.Length)
			me.User = &botapi.User{ID: v.UserID}
		case *telegram.MessageEntityPhone:
			me.Type, me.Offset, me.Length = "phone_number", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityUnderline:
			me.Type, me.Offset, me.Length = "underline", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityStrike:
			me.Type, me.Offset, me.Length = "strikethrough", int(v.Offset), int(v.Length)
		case *telegram.MessageEntitySpoiler:
			me.Type, me.Offset, me.Length = "spoiler", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityBlockquote:
			me.Type, me.Offset, me.Length = "blockquote", int(v.Offset), int(v.Length)
			if v.Collapsed {
				me.Type = "expandable_blockquote"
			}
		case *telegram.MessageEntityBankCard:
			me.Type, me.Offset, me.Length = "bank_card", int(v.Offset), int(v.Length)
		case *telegram.MessageEntityCustomEmoji:
			me.Type, me.Offset, me.Length = "custom_emoji", int(v.Offset), int(v.Length)
			me.CustomEmojiID = strconv.FormatInt(v.DocumentID, 10)
		default:
			continue
		}
		out = append(out, me)
	}
	return out
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
