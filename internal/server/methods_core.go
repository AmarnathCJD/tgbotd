package server

import (
	"encoding/json"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

func init() {
	register("getme", getMe)
	register("logout", logOut)
	register("close", closeMethod)
	register("sendmessage", sendMessage)
	register("sendchataction", sendChatAction)
	register("deletemessage", deleteMessage)
	register("deletemessages", deleteMessages)
	register("forwardmessage", forwardMessage)
	register("editmessagetext", editMessageText)
	register("getupdates", getUpdates)
	register("setwebhook", setWebhook)
	register("deletewebhook", deleteWebhook)
	register("getwebhookinfo", getWebhookInfo)
}

func getMe(s *Server, r *Request) (any, error) {
	me := r.Bot.Me()
	if me == nil {
		return nil, botapi.Errorf(401, "Unauthorized")
	}
	return tlate.SelfUser(me), nil
}

func logOut(s *Server, r *Request) (any, error) {
	_, err := r.Bot.Client.AuthLogOut()
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func closeMethod(s *Server, r *Request) (any, error) {
	return true, nil
}

func resolveChatID(r *Request, name string) (any, error) {
	raw, ok := paramRaw(r, name)
	if !ok {
		return nil, botapi.ErrBadRequest("field \"" + name + "\" is required")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		// username / channel handle
		if strings.HasPrefix(s, "@") || strings.HasPrefix(s, "https://t.me/") {
			return s, nil
		}
		// numeric string
		var n int64
		if _, err := jsonParseInt(strings.TrimPrefix(s, "@"), &n); err == nil {
			return convertBotAPIID(n), nil
		}
		return s, nil
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return convertBotAPIID(n), nil
	}
	return nil, botapi.ErrBadRequest("bad chat_id")
}

func convertBotAPIID(id int64) int64 { return id }

// normalizeBotAPIHTML rewrites Bot API's HTML aliases into the tags gogram's
// formatter recognises. Bot API uses <tg-spoiler>, <tg-emoji emoji-id="...">
// and <blockquote expandable> which gogram doesn't parse directly.
func normalizeBotAPIHTML(text string) string {
	text = strings.ReplaceAll(text, "<tg-spoiler>", "<spoiler>")
	text = strings.ReplaceAll(text, "</tg-spoiler>", "</spoiler>")
	text = strings.ReplaceAll(text, "<blockquote expandable>", "<blockquote>")
	return text
}

func normalizeText(text, parseMode string) string {
	switch strings.ToLower(parseMode) {
	case "html":
		return normalizeBotAPIHTML(text)
	}
	return text
}

func sendMessage(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	text, err := requireString(r, "text")
	if err != nil {
		return nil, err
	}
	opts := &telegram.SendOptions{LinkPreview: true}
	if pm, ok := paramString(r, "parse_mode"); ok {
		opts.ParseMode = pm
		text = normalizeText(text, pm)
	}
	if silent, ok := paramBool(r, "disable_notification"); ok && silent {
		opts.Silent = true
	}
	if protect, ok := paramBool(r, "protect_content"); ok && protect {
		opts.NoForwards = true
	}
	// reply
	if replyRaw, ok := paramRaw(r, "reply_parameters"); ok {
		var rp struct {
			MessageID int32 `json:"message_id"`
		}
		if err := json.Unmarshal(replyRaw, &rp); err == nil && rp.MessageID != 0 {
			opts.ReplyID = rp.MessageID
		}
	}
	if replyID, ok := paramInt64(r, "reply_to_message_id"); ok {
		opts.ReplyID = toInt32Safe(replyID)
	}
	if threadID, ok := paramInt64(r, "message_thread_id"); ok {
		opts.TopicID = toInt32Safe(threadID)
	}
	if effect, ok := paramString(r, "message_effect_id"); ok {
		var n int64
		if _, err := jsonParseInt(effect, &n); err == nil {
			opts.Effect = n
		}
	}
	// keyboards
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	if lpo, ok := paramRaw(r, "link_preview_options"); ok && len(lpo) > 0 {
		var lp struct {
			IsDisabled bool `json:"is_disabled"`
		}
		if err := json.Unmarshal(lpo, &lp); err == nil {
			opts.LinkPreview = !lp.IsDisabled
		}
	}
	if dw, ok := paramBool(r, "disable_web_page_preview"); ok && dw {
		opts.LinkPreview = false
	}

	nm, err := r.Bot.Client.SendMessage(peer, text, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func parseReplyMarkup(raw json.RawMessage) telegram.ReplyMarkup {
	var probe struct {
		InlineKeyboard [][]json.RawMessage `json:"inline_keyboard,omitempty"`
		Keyboard       [][]json.RawMessage `json:"keyboard,omitempty"`
		RemoveKeyboard bool                `json:"remove_keyboard,omitempty"`
		ForceReply     bool                `json:"force_reply,omitempty"`
		Selective      bool                `json:"selective,omitempty"`
		Resize         bool                `json:"resize_keyboard,omitempty"`
		OneTime        bool                `json:"one_time_keyboard,omitempty"`
		Placeholder    string              `json:"input_field_placeholder,omitempty"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	if len(probe.InlineKeyboard) > 0 {
		rows := make([]*telegram.KeyboardButtonRow, 0, len(probe.InlineKeyboard))
		for _, row := range probe.InlineKeyboard {
			buttons := make([]telegram.KeyboardButton, 0, len(row))
			for _, rb := range row {
				if btn := parseInlineButton(rb); btn != nil {
					buttons = append(buttons, btn)
				}
			}
			if len(buttons) > 0 {
				rows = append(rows, &telegram.KeyboardButtonRow{Buttons: buttons})
			}
		}
		return &telegram.ReplyInlineMarkup{Rows: rows}
	}
	if len(probe.Keyboard) > 0 {
		rows := make([]*telegram.KeyboardButtonRow, 0, len(probe.Keyboard))
		for _, row := range probe.Keyboard {
			buttons := make([]telegram.KeyboardButton, 0, len(row))
			for _, rb := range row {
				if btn := parseReplyButton(rb); btn != nil {
					buttons = append(buttons, btn)
				}
			}
			if len(buttons) > 0 {
				rows = append(rows, &telegram.KeyboardButtonRow{Buttons: buttons})
			}
		}
		return &telegram.ReplyKeyboardMarkup{
			Rows:        rows,
			Resize:      probe.Resize,
			SingleUse:   probe.OneTime,
			Selective:   probe.Selective,
			Placeholder: probe.Placeholder,
		}
	}
	if probe.RemoveKeyboard {
		return &telegram.ReplyKeyboardHide{Selective: probe.Selective}
	}
	if probe.ForceReply {
		return &telegram.ReplyKeyboardForceReply{Selective: probe.Selective, Placeholder: probe.Placeholder}
	}
	return nil
}

func parseInlineButton(raw json.RawMessage) telegram.KeyboardButton {
	var b struct {
		Text                          string          `json:"text"`
		URL                           string          `json:"url,omitempty"`
		CallbackData                  string          `json:"callback_data,omitempty"`
		WebApp                        *struct{ URL string } `json:"web_app,omitempty"`
		LoginURL                      *struct{ URL, ForwardText, BotUsername string; RequestWriteAccess bool } `json:"login_url,omitempty"`
		SwitchInlineQuery             *string         `json:"switch_inline_query,omitempty"`
		SwitchInlineQueryCurrentChat  *string         `json:"switch_inline_query_current_chat,omitempty"`
		CopyText                      *struct{ Text string } `json:"copy_text,omitempty"`
		Pay                           bool            `json:"pay,omitempty"`
		CallbackGame                  json.RawMessage `json:"callback_game,omitempty"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil
	}
	switch {
	case b.URL != "":
		return &telegram.KeyboardButtonURL{Text: b.Text, URL: b.URL}
	case b.CallbackData != "":
		return &telegram.KeyboardButtonCallback{Text: b.Text, Data: []byte(b.CallbackData)}
	case b.WebApp != nil:
		return &telegram.KeyboardButtonSimpleWebView{Text: b.Text, URL: b.WebApp.URL}
	case b.LoginURL != nil:
		return &telegram.InputKeyboardButtonURLAuth{
			Text:               b.Text,
			URL:                b.LoginURL.URL,
			FwdText:            b.LoginURL.ForwardText,
			RequestWriteAccess: b.LoginURL.RequestWriteAccess,
			Bot:                &telegram.InputUserSelf{},
		}
	case b.SwitchInlineQuery != nil:
		return &telegram.KeyboardButtonSwitchInline{Text: b.Text, Query: *b.SwitchInlineQuery}
	case b.SwitchInlineQueryCurrentChat != nil:
		return &telegram.KeyboardButtonSwitchInline{Text: b.Text, Query: *b.SwitchInlineQueryCurrentChat, SamePeer: true}
	case b.CopyText != nil:
		return &telegram.KeyboardButtonCopy{Text: b.Text, CopyText: b.CopyText.Text}
	case b.Pay:
		return &telegram.KeyboardButtonBuy{Text: b.Text}
	case len(b.CallbackGame) > 0:
		return &telegram.KeyboardButtonGame{Text: b.Text}
	}
	return nil
}

func parseReplyButton(raw json.RawMessage) telegram.KeyboardButton {
	var b struct {
		Text            string `json:"text"`
		RequestContact  bool   `json:"request_contact,omitempty"`
		RequestLocation bool   `json:"request_location,omitempty"`
		RequestPoll     *struct{ Type string } `json:"request_poll,omitempty"`
		WebApp          *struct{ URL string } `json:"web_app,omitempty"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil
	}
	switch {
	case b.RequestContact:
		return &telegram.KeyboardButtonRequestPhone{Text: b.Text}
	case b.RequestLocation:
		return &telegram.KeyboardButtonRequestGeoLocation{Text: b.Text}
	case b.RequestPoll != nil:
		return &telegram.KeyboardButtonRequestPoll{Text: b.Text}
	case b.WebApp != nil:
		return &telegram.KeyboardButtonSimpleWebView{Text: b.Text, URL: b.WebApp.URL}
	}
	return &telegram.KeyboardButtonObj{Text: b.Text}
}

func newMessageToObj(nm *telegram.NewMessage) *telegram.MessageObj {
	if nm == nil {
		return nil
	}
	return nm.Message
}

func extractUsersChats(nm *telegram.NewMessage) (map[int64]*telegram.UserObj, map[int64]telegram.Chat) {
	users := map[int64]*telegram.UserObj{}
	chats := map[int64]telegram.Chat{}
	if nm == nil {
		return users, chats
	}
	if u := nm.Sender; u != nil {
		users[u.ID] = u
	}
	if c := nm.Channel; c != nil {
		chats[c.ID] = c
	}
	return users, chats
}
