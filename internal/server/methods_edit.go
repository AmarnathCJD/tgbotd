package server

import (
	"encoding/json"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

// deleteMessage → messages.deleteMessages / channels.deleteMessages
func deleteMessage(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.DeleteMessages(peer, []int32{int32(id)}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

// deleteMessages → batch
func deleteMessages(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "message_ids")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"message_ids\" is required")
	}
	var ids []int32
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, botapi.ErrBadRequest("message_ids must be an array of integers")
	}
	if _, err := r.Bot.Client.DeleteMessages(peer, ids); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

// forwardMessage
func forwardMessage(s *Server, r *Request) (any, error) {
	dest, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	src, err := resolveChatID(r, "from_chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.ForwardOptions{}
	if silent, _ := paramBool(r, "disable_notification"); silent {
		opts.Silent = true
	}
	if protect, _ := paramBool(r, "protect_content"); protect {
		opts.Noforwards = true
	}
	msgs, err := r.Bot.Client.Forward(dest, src, []int32{int32(id)}, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if len(msgs) == 0 {
		return nil, botapi.Errorf(500, "forward returned no messages")
	}
	first := msgs[0]
	return tlate.MessageObjToBotAPICtx(newMessageToObj(&first), r.Bot.BuildTranslateContext()), nil
}

// editMessageText
func editMessageText(s *Server, r *Request) (any, error) {
	text, err := requireString(r, "text")
	if err != nil {
		return nil, err
	}
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		return editInlineMessage(r, inline, text, nil, nil)
	}
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.SendOptions{}
	if pm, ok := paramString(r, "parse_mode"); ok {
		opts.ParseMode = pm
	}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	nm, err := r.Bot.Client.EditMessage(peer, int32(id), text, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

// sendChatAction → messages.setTyping
func sendChatAction(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	action, err := requireString(r, "action")
	if err != nil {
		return nil, err
	}
	var act telegram.SendMessageAction
	switch action {
	case "typing":
		act = &telegram.SendMessageTypingAction{}
	case "upload_photo":
		act = &telegram.SendMessageUploadPhotoAction{}
	case "record_video":
		act = &telegram.SendMessageRecordVideoAction{}
	case "upload_video":
		act = &telegram.SendMessageUploadVideoAction{}
	case "record_voice":
		act = &telegram.SendMessageRecordAudioAction{}
	case "upload_voice":
		act = &telegram.SendMessageUploadAudioAction{}
	case "upload_document":
		act = &telegram.SendMessageUploadDocumentAction{}
	case "find_location":
		act = &telegram.SendMessageGeoLocationAction{}
	case "record_video_note":
		act = &telegram.SendMessageRecordRoundAction{}
	case "upload_video_note":
		act = &telegram.SendMessageUploadRoundAction{}
	case "choose_sticker":
		act = &telegram.SendMessageChooseStickerAction{}
	default:
		return nil, botapi.ErrBadRequest("unknown action \"" + action + "\"")
	}
	// SendAction expects **SendMessageAction; bypass its type-switch and call
	// MessagesSetTyping directly with our concrete action.
	ipeer, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesSetTyping(ipeer, 0, act); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
