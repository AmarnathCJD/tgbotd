package server

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/fileid"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

func init() {
	register("copymessage", copyMessage)
	register("copymessages", copyMessages)
	register("editmessagecaption", editMessageCaption)
	register("editmessagereplymarkup", editMessageReplyMarkup)
	register("editmessagemedia", editMessageMedia)
	register("editmessagelivelocation", editMessageLiveLocation)
	register("stopmessagelivelocation", stopMessageLiveLocation)
	register("stoppoll", stopPoll)
	register("setmessagereaction", setMessageReaction)
	register("getuserprofilephotos", getUserProfilePhotos)
	register("setchatdescription", setChatDescription)
	register("setchatphoto", setChatPhoto)
	register("deletechatphoto", deleteChatPhoto)
	register("setchatpermissions", setChatPermissions)
	register("unpinallchatmessages", unpinAllChatMessages)
	register("createchatinvitelink", createChatInviteLink)
	register("editchatinvitelink", editChatInviteLink)
	register("revokechatinvitelink", revokeChatInviteLink)
	register("approvechatjoinrequest", approveChatJoinRequest)
	register("declinechatjoinrequest", declineChatJoinRequest)
	register("createforumtopic", createForumTopic)
	register("editforumtopic", editForumTopic)
	register("closeforumtopic", closeForumTopic)
	register("reopenforumtopic", reopenForumTopic)
	register("deleteforumtopic", deleteForumTopic)
	register("closegeneralforumtopic", closeGeneralForumTopic)
	register("reopengeneralforumtopic", reopenGeneralForumTopic)
	register("hidegeneralforumtopic", hideGeneralForumTopic)
	register("unhidegeneralforumtopic", unhideGeneralForumTopic)
}

// copyMessage: forward with HideAuthor=true (behaves like Bot API copy)
func copyMessage(s *Server, r *Request) (any, error) {
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
	opts := &telegram.ForwardOptions{HideAuthor: true}
	if silent, _ := paramBool(r, "disable_notification"); silent {
		opts.Silent = true
	}
	if protect, _ := paramBool(r, "protect_content"); protect {
		opts.Noforwards = true
	}
	if replyID, ok := paramInt64(r, "reply_to_message_id"); ok {
		opts.ReplyID = int32(replyID)
	}
	if threadID, ok := paramInt64(r, "message_thread_id"); ok {
		opts.TopicID = int32(threadID)
	}
	msgs, err := r.Bot.Client.Forward(dest, src, []int32{int32(id)}, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if len(msgs) == 0 {
		return nil, botapi.Errorf(500, "copy returned no messages")
	}
	// Bot API returns MessageId, not Message.
	return &botapi.MessageID{MessageID: int64(msgs[0].ID)}, nil
}

func copyMessages(s *Server, r *Request) (any, error) {
	dest, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	src, err := resolveChatID(r, "from_chat_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "message_ids")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"message_ids\" is required")
	}
	var ids []int32
	if err := jsonUnmarshalInts(raw, &ids); err != nil {
		return nil, err
	}
	opts := &telegram.ForwardOptions{HideAuthor: true}
	if silent, _ := paramBool(r, "disable_notification"); silent {
		opts.Silent = true
	}
	if protect, _ := paramBool(r, "protect_content"); protect {
		opts.Noforwards = true
	}
	msgs, err := r.Bot.Client.Forward(dest, src, ids, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := make([]botapi.MessageID, len(msgs))
	for i, m := range msgs {
		out[i] = botapi.MessageID{MessageID: int64(m.ID)}
	}
	return out, nil
}

func editMessageCaption(s *Server, r *Request) (any, error) {
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		caption, _ := paramString(r, "caption")
		return editInlineMessage(r, inline, caption, nil, nil)
	}
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	caption, _ := paramString(r, "caption")
	opts := &telegram.SendOptions{}
	if pm, ok := paramString(r, "parse_mode"); ok {
		opts.ParseMode = pm
	}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	nm, err := r.Bot.Client.EditMessage(peer, int32(id), caption, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func editMessageReplyMarkup(s *Server, r *Request) (any, error) {
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		return editInlineMessage(r, inline, "", nil, nil)
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
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	nm, err := r.Bot.Client.EditMessage(peer, int32(id), "", opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

// editMessageMedia — the media param is an InputMedia object; we resolve
// its "media" field to file_id/URL/attach:// like sendMediaGroup does.
func editMessageMedia(s *Server, r *Request) (any, error) {
	rawEarly, hasMedia := paramRaw(r, "media")
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		if !hasMedia {
			return nil, botapi.ErrBadRequest("field \"media\" is required")
		}
		var m struct {
			Type    string `json:"type"`
			Media   string `json:"media"`
			Caption string `json:"caption,omitempty"`
		}
		if err := jsonUnmarshalRaw(rawEarly, &m); err != nil {
			return nil, botapi.ErrBadRequest("bad media")
		}
		var im telegram.InputMedia
		if len(m.Media) > 9 && m.Media[:9] == "attach://" {
			// attach:// not resolvable to a URL/file_id — fall back to
			// standard InputMedia URL/file_id path.
			im = &telegram.InputMediaPhotoExternal{URL: m.Media}
		} else {
			switch m.Type {
			case "video", "document", "audio", "voice", "animation":
				im = &telegram.InputMediaDocumentExternal{URL: m.Media}
			default:
				im = &telegram.InputMediaPhotoExternal{URL: m.Media}
			}
		}
		return editInlineMessage(r, inline, "", im, nil)
	}
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "media")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"media\" is required")
	}
	var m struct {
		Type    string `json:"type"`
		Media   string `json:"media"`
		Caption string `json:"caption,omitempty"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, botapi.ErrBadRequest("bad media")
	}
	var mediaVal any
	var tmp string
	if strings.HasPrefix(m.Media, "attach://") {
		if fh, ok := r.Files[strings.TrimPrefix(m.Media, "attach://")]; ok {
			mediaVal, tmp, err = handleMultipartFile(fh)
			if err != nil {
				return nil, err
			}
			if tmp != "" {
				defer os.Remove(tmp)
			}
		} else {
			return nil, botapi.ErrBadRequest("attach:// with no matching part")
		}
	} else {
		mediaVal = m.Media
	}
	opts := &telegram.SendOptions{}
	if m.Caption != "" {
		opts.Caption = m.Caption
	}
	if pm, _ := paramString(r, "parse_mode"); pm != "" {
		opts.ParseMode = pm
	}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	nm, err := r.Bot.Client.EditMessage(peer, int32(id), mediaVal, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func editMessageLiveLocation(s *Server, r *Request) (any, error) {
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		lat, lon, err := requireLatLon(r)
		if err != nil {
			return nil, err
		}
		live := &telegram.InputMediaGeoLive{
			GeoPoint: &telegram.InputGeoPointObj{Lat: lat, Long: lon},
		}
		if lp, ok := paramInt64(r, "live_period"); ok {
			live.Period = int32(lp)
		}
		if h, ok := paramInt64(r, "heading"); ok {
			live.Heading = int32(h)
		}
		if pr, ok := paramInt64(r, "proximity_alert_radius"); ok {
			live.ProximityNotificationRadius = int32(pr)
		}
		return editInlineMessage(r, inline, "", live, nil)
	}
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	lat, lon, err := requireLatLon(r)
	if err != nil {
		return nil, err
	}
	geo := &telegram.InputGeoPointObj{Lat: lat, Long: lon}
	live := &telegram.InputMediaGeoLive{GeoPoint: geo}
	if lp, ok := paramInt64(r, "live_period"); ok {
		live.Period = int32(lp)
	}
	if h, ok := paramInt64(r, "heading"); ok {
		live.Heading = int32(h)
	}
	if r2, ok := paramInt64(r, "proximity_alert_radius"); ok {
		live.ProximityNotificationRadius = int32(r2)
	}
	opts := &telegram.SendOptions{}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	nm, err := r.Bot.Client.EditMessage(peer, int32(id), live, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func stopMessageLiveLocation(s *Server, r *Request) (any, error) {
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		live := &telegram.InputMediaGeoLive{Stopped: true}
		return editInlineMessage(r, inline, "", live, nil)
	}
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	live := &telegram.InputMediaGeoLive{Stopped: true}
	nm, err := r.Bot.Client.EditMessage(peer, int32(id), live)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func stopPoll(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	original, err := r.Bot.Client.GetMessageByID(peer, int32(id))
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	mo := newMessageToObj(original)
	if mo == nil || mo.Media == nil {
		return nil, botapi.ErrBadRequest("message is not a poll")
	}
	pollMedia, ok := mo.Media.(*telegram.MessageMediaPoll)
	if !ok || pollMedia.Poll == nil {
		return nil, botapi.ErrBadRequest("message is not a poll")
	}
	closedPoll := *pollMedia.Poll
	closedPoll.Closed = true
	closed := &telegram.InputMediaPoll{Poll: &closedPoll}
	opts := &telegram.SendOptions{}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	nm, err := r.Bot.Client.EditMessage(peer, int32(id), closed, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	tctx := r.Bot.BuildTranslateContext()
	bm := tlate.MessageObjToBotAPICtx(newMessageToObj(nm), tctx)
	if bm != nil && bm.Poll != nil {
		return bm.Poll, nil
	}
	return map[string]any{"is_closed": true}, nil
}

// clearMessageReaction removes any reaction on a message. gogram's
// SendReaction rejects nil via its convertReaction switch — call
// MessagesSendReaction directly with an empty []Reaction instead.
func clearMessageReaction(r *Request, peer any, msgID int32) (any, error) {
	ipeer, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesSendReaction(&telegram.MessagesSendReactionParams{
		Peer:  ipeer,
		MsgID: msgID,
	}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setMessageReaction(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "reaction")
	if !ok || len(raw) == 0 {
		return clearMessageReaction(r, peer, int32(id))
	}
	var entries []struct {
		Type          string `json:"type"`
		Emoji         string `json:"emoji,omitempty"`
		CustomEmojiID string `json:"custom_emoji_id,omitempty"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, botapi.ErrBadRequest("reaction must be an array of {type, emoji}")
	}
	if len(entries) == 0 {
		return clearMessageReaction(r, peer, int32(id))
	}
	// gogram accepts string or []string for emoji reactions.
	emojis := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type == "emoji" && e.Emoji != "" {
			emojis = append(emojis, e.Emoji)
		}
	}
	if len(emojis) == 0 {
		return true, nil
	}
	isBig, _ := paramBool(r, "is_big")
	if err := r.Bot.Client.SendReaction(peer, int32(id), emojis, isBig); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func getUserProfilePhotos(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	limit := int32(100)
	if n, ok := paramInt64(r, "limit"); ok && n > 0 && n <= 100 {
		limit = int32(n)
	}
	photos, err := r.Bot.Client.GetChatPhotos(uid, limit)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	// Translate each Photo (variant) into a Bot API []PhotoSize array. Bot API
	// returns photos as [][]PhotoSize — one PhotoSize array per photo, with
	// each element being a different rendered size.
	out := make([][]map[string]any, 0, len(photos))
	for _, p := range photos {
		po, ok := p.(*telegram.PhotoObj)
		if !ok {
			continue
		}
		one := make([]map[string]any, 0, len(po.Sizes))
		for _, sz := range po.Sizes {
			ps, ok := sz.(*telegram.PhotoSizeObj)
			if !ok {
				continue
			}
			info := &fileid.Info{
				DC:         po.DcID,
				Type:       fileid.FTPhoto,
				ID:         po.ID,
				AccessHash: po.AccessHash,
				FileRef:    po.FileReference,
			}
			one = append(one, map[string]any{
				"file_id":        info.Encode(),
				"file_unique_id": info.UniqueID(),
				"width":          ps.W,
				"height":         ps.H,
				"file_size":      ps.Size,
			})
		}
		out = append(out, one)
	}
	return map[string]any{
		"total_count": len(photos),
		"photos":      out,
	}, nil
}

// chat modification

func setChatDescription(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	desc, _ := paramString(r, "description")
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesEditChatAbout(p, desc); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setChatPhoto(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	src, tmp, err := resolveInputFile(r, "photo")
	if err != nil {
		return nil, err
	}
	if tmp != "" {
		defer os.Remove(tmp)
	}
	up, err := r.Bot.Client.UploadFile(src)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	inputPhoto := &telegram.InputChatUploadedPhoto{File: up}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if inputCh, ok := p.(*telegram.InputPeerChannel); ok {
		if _, err := r.Bot.Client.ChannelsEditPhoto(
			&telegram.InputChannelObj{ChannelID: inputCh.ChannelID, AccessHash: inputCh.AccessHash},
			inputPhoto,
		); err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return true, nil
	}
	if inputChat, ok := p.(*telegram.InputPeerChat); ok {
		if _, err := r.Bot.Client.MessagesEditChatPhoto(inputChat.ChatID, inputPhoto); err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return true, nil
	}
	return nil, botapi.ErrBadRequest("chat_id must be a group/supergroup/channel")
}

func deleteChatPhoto(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	empty := &telegram.InputChatPhotoEmpty{}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if inputCh, ok := p.(*telegram.InputPeerChannel); ok {
		if _, err := r.Bot.Client.ChannelsEditPhoto(
			&telegram.InputChannelObj{ChannelID: inputCh.ChannelID, AccessHash: inputCh.AccessHash},
			empty,
		); err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return true, nil
	}
	if inputChat, ok := p.(*telegram.InputPeerChat); ok {
		if _, err := r.Bot.Client.MessagesEditChatPhoto(inputChat.ChatID, empty); err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return true, nil
	}
	return nil, botapi.ErrBadRequest("chat_id must be a group/supergroup/channel")
}

func setChatPermissions(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	rights := &telegram.ChatBannedRights{}
	if raw, ok := paramRaw(r, "permissions"); ok && len(raw) > 0 {
		var perm struct {
			CanSendMessages       *bool `json:"can_send_messages"`
			CanSendMediaMessages  *bool `json:"can_send_media_messages"`
			CanSendPolls          *bool `json:"can_send_polls"`
			CanSendOtherMessages  *bool `json:"can_send_other_messages"`
			CanAddWebPagePreviews *bool `json:"can_add_web_page_previews"`
			CanChangeInfo         *bool `json:"can_change_info"`
			CanInviteUsers        *bool `json:"can_invite_users"`
			CanPinMessages        *bool `json:"can_pin_messages"`
			CanManageTopics       *bool `json:"can_manage_topics"`
		}
		if err := json.Unmarshal(raw, &perm); err == nil {
			neg := func(b *bool) bool { return b != nil && !*b }
			rights.SendMessages = neg(perm.CanSendMessages)
			rights.SendMedia = neg(perm.CanSendMediaMessages)
			rights.SendPolls = neg(perm.CanSendPolls)
			rights.SendStickers = neg(perm.CanSendOtherMessages)
			rights.SendGifs = neg(perm.CanSendOtherMessages)
			rights.SendInline = neg(perm.CanSendOtherMessages)
			rights.EmbedLinks = neg(perm.CanAddWebPagePreviews)
			rights.ChangeInfo = neg(perm.CanChangeInfo)
			rights.InviteUsers = neg(perm.CanInviteUsers)
			rights.PinMessages = neg(perm.CanPinMessages)
			rights.ManageTopics = neg(perm.CanManageTopics)
		}
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesEditChatDefaultBannedRights(p, rights); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func unpinAllChatMessages(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.UnpinMessage(peer, 0, &telegram.PinOptions{Unpin: true}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

// invite link methods

func createChatInviteLink(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.InviteLinkOptions{}
	if name, ok := paramString(r, "name"); ok {
		opts.Title = name
	}
	if exp, ok := paramInt64(r, "expire_date"); ok {
		opts.Expire = int32(exp)
	}
	if lim, ok := paramInt64(r, "member_limit"); ok {
		opts.Limit = int32(lim)
	}
	if req, _ := paramBool(r, "creates_join_request"); req {
		opts.RequestNeeded = true
	}
	link, err := r.Bot.Client.GetChatInviteLink(peer, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return inviteLinkToBotAPI(link), nil
}

func editChatInviteLink(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	inviteLink, err := requireString(r, "invite_link")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	params := &telegram.MessagesEditExportedChatInviteParams{
		Peer: p,
		Link: inviteLink,
	}
	if name, ok := paramString(r, "name"); ok {
		params.Title = name
	}
	if exp, ok := paramInt64(r, "expire_date"); ok {
		params.ExpireDate = int32(exp)
	}
	if lim, ok := paramInt64(r, "member_limit"); ok {
		params.UsageLimit = int32(lim)
	}
	if req, _ := paramBool(r, "creates_join_request"); req {
		params.RequestNeeded = true
	}
	res, err := r.Bot.Client.MessagesEditExportedChatInvite(params)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if obj, ok := res.(*telegram.MessagesExportedChatInviteObj); ok {
		return inviteLinkToBotAPI(obj.Invite), nil
	}
	return map[string]any{"invite_link": inviteLink}, nil
}

func revokeChatInviteLink(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	link, err := requireString(r, "invite_link")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.RevokeInvite(peer, link); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return map[string]any{
		"invite_link": link,
		"is_revoked":  true,
	}, nil
}

func inviteLinkToBotAPI(link telegram.ExportedChatInvite) map[string]any {
	if l, ok := link.(*telegram.ChatInviteExported); ok {
		return map[string]any{
			"invite_link":               l.Link,
			"name":                      l.Title,
			"is_primary":                l.Permanent,
			"is_revoked":                l.Revoked,
			"creates_join_request":      l.RequestNeeded,
			"expire_date":               l.ExpireDate,
			"member_limit":              l.UsageLimit,
			"pending_join_request_count": l.Requested,
		}
	}
	return map[string]any{}
}

// join request approvals

func approveChatJoinRequest(s *Server, r *Request) (any, error) {
	// gogram: MessagesHideChatJoinRequest with approved=true
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	inpPeer, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	usr, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesHideChatJoinRequest(true, inpPeer, inputUserFromPeer(usr)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func declineChatJoinRequest(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	inpPeer, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	usr, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesHideChatJoinRequest(false, inpPeer, inputUserFromPeer(usr)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func inputUserFromPeer(p telegram.InputPeer) telegram.InputUser {
	if pu, ok := p.(*telegram.InputPeerUser); ok {
		return &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}
	}
	return &telegram.InputUserEmpty{}
}

// forum topics

func createForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	title, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	opts := &telegram.CreateTopicOptions{}
	if color, ok := paramInt64(r, "icon_color"); ok {
		opts.IconColor = int32(color)
	}
	if emoji, ok := paramString(r, "icon_custom_emoji_id"); ok {
		var n int64
		if _, err := jsonParseInt(emoji, &n); err == nil {
			opts.IconEmojiID = n
		}
	}
	topicID, err := r.Bot.Client.CreateTopic(peer, title, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return map[string]any{
		"message_thread_id":   topicID,
		"name":                title,
		"icon_color":          opts.IconColor,
		"icon_custom_emoji_id": "",
	}, nil
}

func editForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	threadID, err := requireInt64(r, "message_thread_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.EditTopicOptions{}
	if title, ok := paramString(r, "name"); ok {
		opts.Title = title
	}
	if err := r.Bot.Client.EditTopic(peer, int32(threadID), opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func closeForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	threadID, err := requireInt64(r, "message_thread_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.CloseTopic(peer, int32(threadID)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func reopenForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	threadID, err := requireInt64(r, "message_thread_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.ReopenTopic(peer, int32(threadID)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func deleteForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	threadID, err := requireInt64(r, "message_thread_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.DeleteTopic(peer, int32(threadID)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func closeGeneralForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.CloseTopic(peer, 1); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func reopenGeneralForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.ReopenTopic(peer, 1); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func hideGeneralForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.HideTopic(peer, 1); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func unhideGeneralForumTopic(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.UnhideTopic(peer, 1); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
