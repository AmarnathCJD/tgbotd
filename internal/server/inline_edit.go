package server

import (
	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botmgr"
)

// editInlineMessage centralises the messages.editInlineBotMessage plumbing
// used by every edit* Bot API method's inline branch.
//
// Exactly one of text / media / (nil, nil) may be non-nil:
//   - non-empty text → edits the message text
//   - non-nil media  → edits the media (InputMedia)
//   - both nil       → only reply_markup / caption is being changed
//
// The reply_markup and parse_mode / entities are pulled from the request.
func editInlineMessage(r *Request, inlineID string, text any, media telegram.InputMedia, extraOpts func(*telegram.MessagesEditInlineBotMessageParams)) (any, error) {
	id, err := decodeInlineMessageID(inlineID)
	if err != nil {
		return nil, err
	}
	params := &telegram.MessagesEditInlineBotMessageParams{ID: id}
	switch v := text.(type) {
	case string:
		if v != "" {
			params.Message = v
		}
	}
	if media != nil {
		params.Media = media
	}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		params.ReplyMarkup = parseReplyMarkup(kb)
	}
	if raw, ok := paramRaw(r, "link_preview_options"); ok && len(raw) > 0 {
		var lp struct {
			IsDisabled bool `json:"is_disabled"`
		}
		if err := jsonUnmarshalRaw(raw, &lp); err == nil && lp.IsDisabled {
			params.NoWebpage = true
		}
	}
	if dw, ok := paramBool(r, "disable_web_page_preview"); ok && dw {
		params.NoWebpage = true
	}
	if showAbove, ok := paramBool(r, "show_caption_above_media"); ok && showAbove {
		params.InvertMedia = true
	}
	if extraOpts != nil {
		extraOpts(params)
	}
	if _, err := r.Bot.Client.MessagesEditInlineBotMessage(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	// Bot API returns True on success for inline edits.
	return true, nil
}
