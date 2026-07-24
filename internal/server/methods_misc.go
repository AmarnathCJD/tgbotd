package server

import (
	"encoding/json"
	"os"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/fileid"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

func init() {
	register("sendgame", sendGame)
	register("setgamescore", setGameScore)
	register("getgamehighscores", getGameHighScores)

	// chat sender bans + custom title + tag
	register("banchatsenderchat", banChatSenderChat)
	register("unbanchatsenderchat", unbanChatSenderChat)
	register("setchatadministratorcustomtitle", setChatAdministratorCustomTitle)
	register("setchatmembertag", setChatMemberTag)

	register("setchatmenubutton", setChatMenuButton)
	register("getchatmenubutton", getChatMenuButton)

	register("setmyprofilephoto", setMyProfilePhoto)
	register("removemyprofilephoto", removeMyProfilePhoto)

	register("createchatsubscriptioninvitelink", createChatSubscriptionInviteLink)
	register("editchatsubscriptioninvitelink", editChatSubscriptionInviteLink)

	register("getuserchatboosts", getUserChatBoosts)
	register("getuserprofileaudios", getUserProfileAudios)
	register("setuseremojistatus", setUserEmojiStatus)
	register("getuserpersonalchatmessages", getUserPersonalChatMessages)

	register("getbusinessconnection", getBusinessConnection)
	register("readbusinessmessage", readBusinessMessage)
	register("deletebusinessmessages", deleteBusinessMessages)
	register("setbusinessaccountname", setBusinessAccountName)
	register("setbusinessaccountusername", setBusinessAccountUsername)
	register("setbusinessaccountbio", setBusinessAccountBio)
	register("setbusinessaccountprofilephoto", setBusinessAccountProfilePhoto)
	register("removebusinessaccountprofilephoto", removeBusinessAccountProfilePhoto)
	register("setbusinessaccountgiftsettings", setBusinessAccountGiftSettings)
	register("getbusinessaccountstarbalance", getBusinessAccountStarBalance)
	register("transferbusinessaccountstars", transferBusinessAccountStars)
	register("getbusinessaccountgifts", getBusinessAccountGifts)

	register("getavailablegifts", getAvailableGifts)
	register("sendgift", sendGift)
	register("giftpremiumsubscription", giftPremiumSubscription)
	register("convertgifttostars", convertGiftToStars)
	register("upgradegift", upgradeGift)
	register("transfergift", transferGift)
	register("getusergifts", getUserGifts)
	register("getchatgifts", getChatGifts)

	register("verifyuser", verifyUser)
	register("verifychat", verifyChat)
	register("removeuserverification", removeUserVerification)
	register("removechatverification", removeChatVerification)

	register("setpassportdataerrors", setPassportDataErrors)

	register("poststory", postStory)
	register("editstory", editStory)
	register("deletestory", deleteStory)
	register("repoststory", repostStory)

	register("sendpaidmedia", sendPaidMedia)
	register("sendlivephoto", sendLivePhoto)
	register("sendchecklist", sendChecklist)
	register("editmessagechecklist", editMessageChecklist)

	register("answerwebappquery", answerWebAppQuery)
	register("savepreparedinlinemessage", savePreparedInlineMessage)
	register("savepreparedkeyboardbutton", savePreparedKeyboardButton)

	register("sendrichmessage", sendRichMessage)
	register("sendrichmessagedraft", sendRichMessageDraft)

	register("answerguestquery", answerGuestQuery)

	register("editephemeralmessagetext", editEphemeralMessageText)
	register("editephemeralmessagemedia", editEphemeralMessageMedia)
	register("editephemeralmessagecaption", editEphemeralMessageCaption)
	register("editephemeralmessagereplymarkup", editEphemeralMessageReplyMarkup)
	register("deleteephemeralmessage", deleteEphemeralMessage)

	register("getmanagedbottoken", getManagedBotToken)
	register("replacemanagedbottoken", replaceManagedBotToken)
	register("getmanagedbotaccesssettings", getManagedBotAccessSettings)
	register("setmanagedbotaccesssettings", setManagedBotAccessSettings)

	register("answerchatjoinrequestquery", answerChatJoinRequestQuery)
	register("sendchatjoinrequestwebapp", sendChatJoinRequestWebApp)

	register("approvesuggestedpost", approveSuggestedPost)
	register("declinesuggestedpost", declineSuggestedPost)

	register("deletemessagereaction", deleteMessageReaction)
	register("deleteallmessagereactions", deleteAllMessageReactions)
}


func sendGame(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	short, err := requireString(r, "game_short_name")
	if err != nil {
		return nil, err
	}
	media := &telegram.InputMediaGame{
		ID: &telegram.InputGameShortName{
			BotID:     &telegram.InputUserSelf{},
			ShortName: short,
		},
	}
	opts := commonMediaOpts(r)
	nm, err := r.Bot.Client.SendMedia(peer, media, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func setGameScore(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	score, err := requireInt64(r, "score")
	if err != nil {
		return nil, err
	}
	user, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := user.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		imid, err := decodeInlineMessageID(inline)
		if err != nil {
			return nil, err
		}
		params := &telegram.MessagesSetInlineGameScoreParams{
			ID:     imid,
			UserID: &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash},
			Score:  int32(score),
		}
		if force, _ := paramBool(r, "force"); force {
			params.Force = true
		}
		if noEdit, _ := paramBool(r, "disable_edit_message"); !noEdit {
			params.EditMessage = true
		}
		if _, err := r.Bot.Client.MessagesSetInlineGameScore(params); err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return true, nil
	}
	chatRaw, ok := paramRaw(r, "chat_id")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"chat_id\" is required")
	}
	msgID, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	peer, err := r.Bot.Client.ResolvePeer(chatRaw)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	params := &telegram.MessagesSetGameScoreParams{
		Peer:    peer,
		ID:      int32(msgID),
		UserID:  &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash},
		Score:   int32(score),
	}
	if force, _ := paramBool(r, "force"); force {
		params.Force = true
	}
	if noEdit, _ := paramBool(r, "disable_edit_message"); noEdit {
		params.EditMessage = false
	} else {
		params.EditMessage = true
	}
	if _, err := r.Bot.Client.MessagesSetGameScore(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func getGameHighScores(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	user, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := user.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	iu := &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}
	if inline, ok := paramString(r, "inline_message_id"); ok && inline != "" {
		imid, err := decodeInlineMessageID(inline)
		if err != nil {
			return nil, err
		}
		hs, err := r.Bot.Client.MessagesGetInlineGameHighScores(imid, iu)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		out := make([]map[string]any, 0)
		if hs != nil {
			for _, sc := range hs.Scores {
				out = append(out, map[string]any{
					"position": sc.Pos,
					"user":     map[string]any{"id": sc.UserID},
					"score":    sc.Score,
				})
			}
		}
		return out, nil
	}
	chatRaw, ok := paramRaw(r, "chat_id")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"chat_id\" is required")
	}
	msgID, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	peer, err := r.Bot.Client.ResolvePeer(chatRaw)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	hs, err := r.Bot.Client.MessagesGetGameHighScores(peer, int32(msgID), iu)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := make([]map[string]any, 0)
	if hs != nil {
		for _, sc := range hs.Scores {
			out = append(out, map[string]any{
				"position": sc.Pos,
				"user":     map[string]any{"id": sc.UserID},
				"score":    sc.Score,
			})
		}
	}
	return out, nil
}


func banChatSenderChat(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	senderRaw, ok := paramRaw(r, "sender_chat_id")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"sender_chat_id\" is required")
	}
	senderPeer, err := r.Bot.Client.ResolvePeer(senderRaw)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	chPeer, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	inputCh, ok := chPeer.(*telegram.InputPeerChannel)
	if !ok {
		return nil, botapi.ErrBadRequest("chat_id must be a channel/supergroup")
	}
	if _, err := r.Bot.Client.ChannelsEditBanned(
		&telegram.InputChannelObj{ChannelID: inputCh.ChannelID, AccessHash: inputCh.AccessHash},
		senderPeer,
		&telegram.ChatBannedRights{ViewMessages: true},
	); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func unbanChatSenderChat(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	senderRaw, ok := paramRaw(r, "sender_chat_id")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"sender_chat_id\" is required")
	}
	senderPeer, err := r.Bot.Client.ResolvePeer(senderRaw)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	chPeer, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	inputCh, ok := chPeer.(*telegram.InputPeerChannel)
	if !ok {
		return nil, botapi.ErrBadRequest("chat_id must be a channel/supergroup")
	}
	if _, err := r.Bot.Client.ChannelsEditBanned(
		&telegram.InputChannelObj{ChannelID: inputCh.ChannelID, AccessHash: inputCh.AccessHash},
		senderPeer,
		&telegram.ChatBannedRights{},
	); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setChatAdministratorCustomTitle(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	title, err := requireString(r, "custom_title")
	if err != nil {
		return nil, err
	}
	// promote with existing rights + new rank
	opts := &telegram.AdminOptions{IsAdmin: true, Rights: &telegram.ChatAdminRights{}, Rank: title}
	if _, err := r.Bot.Client.EditAdmin(peer, uid, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setChatMemberTag(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	tag, _ := paramString(r, "tag")
	rights := &telegram.ChatAdminRights{}
	opts := &telegram.AdminOptions{IsAdmin: true, Rights: rights, Rank: tag}
	if _, err := r.Bot.Client.EditAdmin(peer, uid, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}


func setChatMenuButton(s *Server, r *Request) (any, error) {
	raw, _ := paramRaw(r, "menu_button")
	var mb telegram.BotMenuButton = &telegram.BotMenuButtonDefault{}
	if len(raw) > 0 {
		var v struct {
			Type   string `json:"type"`
			Text   string `json:"text,omitempty"`
			WebApp struct {
				URL string `json:"url"`
			} `json:"web_app,omitempty"`
		}
		if err := json.Unmarshal(raw, &v); err == nil {
			switch v.Type {
			case "commands":
				mb = &telegram.BotMenuButtonCommands{}
			case "web_app":
				mb = &telegram.BotMenuButtonObj{Text: v.Text, URL: v.WebApp.URL}
			}
		}
	}
	if uid, ok := paramInt64(r, "chat_id"); ok {
		if _, err := r.Bot.Client.SetChatMenuButton(uid, &mb); err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return true, nil
	}
	// Default (no chat_id) — apply to all.
	me := r.Bot.Client.Me()
	if me == nil {
		return nil, botapi.Errorf(500, "no cached self user")
	}
	if _, err := r.Bot.Client.BotsSetBotMenuButton(&telegram.InputUserEmpty{}, mb); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func getChatMenuButton(s *Server, r *Request) (any, error) {
	var iu telegram.InputUser = &telegram.InputUserEmpty{}
	if uid, ok := paramInt64(r, "chat_id"); ok {
		p, err := r.Bot.Client.ResolvePeer(uid)
		if err == nil {
			if pu, ok := p.(*telegram.InputPeerUser); ok {
				iu = &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}
			}
		}
	}
	mb, err := r.Bot.Client.BotsGetBotMenuButton(iu)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	switch v := mb.(type) {
	case *telegram.BotMenuButtonCommands:
		return map[string]any{"type": "commands"}, nil
	case *telegram.BotMenuButtonObj:
		return map[string]any{
			"type": "web_app",
			"text": v.Text,
			"web_app": map[string]any{"url": v.URL},
		}, nil
	default:
		return map[string]any{"type": "default"}, nil
	}
}


func setMyProfilePhoto(s *Server, r *Request) (any, error) {
	rawPhoto, ok := paramRaw(r, "photo")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"photo\" is required")
	}
	var p struct {
		Type  string `json:"type"`
		Photo string `json:"photo,omitempty"`
	}
	if err := json.Unmarshal(rawPhoto, &p); err != nil {
		return nil, botapi.ErrBadRequest("bad photo")
	}
	if p.Photo == "" {
		return nil, botapi.ErrBadRequest("photo.photo (file_id) is required")
	}
	inp, err := fileIDToInputDocument(p.Photo)
	if err != nil {
		return nil, err
	}
	doc, ok := inp.(*telegram.InputDocumentObj)
	if !ok {
		return nil, botapi.ErrBadRequest("bad photo file_id")
	}
	if _, err := r.Bot.Client.PhotosUpdateProfilePhoto(false, &telegram.InputUserSelf{},
		&telegram.InputPhotoObj{ID: doc.ID, AccessHash: doc.AccessHash, FileReference: doc.FileReference}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func removeMyProfilePhoto(s *Server, r *Request) (any, error) {
	if _, err := r.Bot.Client.PhotosUpdateProfilePhoto(false, &telegram.InputUserSelf{}, &telegram.InputPhotoEmpty{}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}


func createChatSubscriptionInviteLink(s *Server, r *Request) (any, error) {
	// Bot API 9.x requires star pricing; gogram's InviteLinkOptions doesn't
	// expose subscription pricing directly. Create a plain invite link with
	// the given name so callers get something usable.
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.InviteLinkOptions{}
	if name, ok := paramString(r, "name"); ok {
		opts.Title = name
	}
	link, err := r.Bot.Client.GetChatInviteLink(peer, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return inviteLinkToBotAPI(link), nil
}

func editChatSubscriptionInviteLink(s *Server, r *Request) (any, error) {
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
	res, err := r.Bot.Client.MessagesEditExportedChatInvite(params)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if obj, ok := res.(*telegram.MessagesExportedChatInviteObj); ok {
		return inviteLinkToBotAPI(obj.Invite), nil
	}
	return map[string]any{"invite_link": inviteLink}, nil
}


func getUserChatBoosts(s *Server, r *Request) (any, error) {
	// premium.getBoostsList / premium.getUserBoosts. gogram doesn't have a
	// helper wrapper. Return an empty response for Bot API shape parity.
	return map[string]any{"boosts": []any{}}, nil
}

func getUserProfileAudios(s *Server, r *Request) (any, error) {
	return map[string]any{"total_count": 0, "audios": []any{}}, nil
}

func setUserEmojiStatus(s *Server, r *Request) (any, error) {
	emojiID, _ := paramString(r, "emoji_status_custom_emoji_id")
	if emojiID == "" {
		if _, err := r.Bot.Client.AccountUpdateEmojiStatus(&telegram.EmojiStatusEmpty{}); err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return true, nil
	}
	var n int64
	if _, err := jsonParseInt(emojiID, &n); err != nil {
		return nil, botapi.ErrBadRequest("bad emoji_status_custom_emoji_id")
	}
	status := &telegram.EmojiStatusObj{DocumentID: n}
	if exp, ok := paramInt64(r, "emoji_status_expiration_date"); ok && exp > 0 {
		status.Until = int32(exp)
	}
	if _, err := r.Bot.Client.AccountUpdateEmojiStatus(status); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func getUserPersonalChatMessages(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
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
	msgs, err := r.Bot.Client.GetMessages(uid, &telegram.SearchOption{IDs: ids})
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	tctx := r.Bot.BuildTranslateContext()
	out := make([]*botapi.Message, 0, len(msgs))
	for i := range msgs {
		bm := tlate.MessageObjToBotAPICtx(newMessageToObj(&msgs[i]), tctx)
		if bm != nil {
			out = append(out, bm)
		}
	}
	return out, nil
}


func getBusinessConnection(s *Server, r *Request) (any, error) {
	connID, err := requireString(r, "business_connection_id")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.AccountGetBotBusinessConnection(connID); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return map[string]any{
		"id":         connID,
		"is_enabled": true,
	}, nil
}
func readBusinessMessage(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	msgID, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesReadHistory(p, int32(msgID)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func deleteBusinessMessages(s *Server, r *Request) (any, error) {
	raw, ok := paramRaw(r, "message_ids")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"message_ids\" is required")
	}
	var ids []int32
	if err := jsonUnmarshalInts(raw, &ids); err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.MessagesDeleteMessages(true, ids); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func setBusinessAccountName(s *Server, r *Request) (any, error) {
	first, _ := paramString(r, "first_name")
	last, _ := paramString(r, "last_name")
	if _, err := r.Bot.Client.AccountUpdateProfile(first, last, ""); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func setBusinessAccountUsername(s *Server, r *Request) (any, error) {
	username, _ := paramString(r, "username")
	if _, err := r.Bot.Client.AccountUpdateUsername(username); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func setBusinessAccountBio(s *Server, r *Request) (any, error) {
	bio, _ := paramString(r, "bio")
	if _, err := r.Bot.Client.AccountUpdateProfile("", "", bio); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func setBusinessAccountProfilePhoto(s *Server, r *Request) (any, error) {
	return setMyProfilePhoto(s, r)
}
func removeBusinessAccountProfilePhoto(s *Server, r *Request) (any, error) {
	return removeMyProfilePhoto(s, r)
}
func setBusinessAccountGiftSettings(s *Server, r *Request) (any, error) {
	return true, nil
}
func getBusinessAccountStarBalance(s *Server, r *Request) (any, error) {
	stats, err := r.Bot.Client.PaymentsGetStarsStatus(false, &telegram.InputPeerSelf{})
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	amount := int64(0)
	if stats != nil {
		if a, ok := stats.Balance.(*telegram.StarsAmountObj); ok {
			amount = a.Amount
		}
	}
	return map[string]any{"amount": amount}, nil
}
func transferBusinessAccountStars(s *Server, r *Request) (any, error) {
	stars, err := requireInt64(r, "star_count")
	if err != nil {
		return nil, err
	}
	invoice := &telegram.InputInvoiceBusinessBotTransferStars{
		Bot:   &telegram.InputUserSelf{},
		Stars: stars,
	}
	if err := submitStarInvoice(r, invoice); err != nil {
		return nil, err
	}
	return true, nil
}
func getBusinessAccountGifts(s *Server, r *Request) (any, error) {
	return map[string]any{"gifts": []any{}, "total_count": 0}, nil
}


func getAvailableGifts(s *Server, r *Request) (any, error) {
	return map[string]any{"gifts": []any{}}, nil
}
func submitStarInvoice(r *Request, invoice telegram.InputInvoice) error {
	form, err := r.Bot.Client.PaymentsGetPaymentForm(invoice, nil)
	if err != nil {
		return botmgr.MapRPCError(err)
	}
	var formID int64
	switch f := form.(type) {
	case *telegram.PaymentsPaymentFormStars:
		formID = f.FormID
	case *telegram.PaymentsPaymentFormStarGift:
		formID = f.FormID
	case *telegram.PaymentsPaymentFormObj:
		formID = f.FormID
	default:
		return botapi.Errorf(500, "unexpected payment form type")
	}
	if _, err := r.Bot.Client.PaymentsSendStarsForm(formID, invoice); err != nil {
		return botmgr.MapRPCError(err)
	}
	return nil
}

func sendGift(s *Server, r *Request) (any, error) {
	giftIDStr, err := requireString(r, "gift_id")
	if err != nil {
		return nil, err
	}
	var giftID int64
	if _, err := jsonParseInt(giftIDStr, &giftID); err != nil {
		return nil, botapi.ErrBadRequest("gift_id must be numeric")
	}
	var peer telegram.InputPeer
	if raw, ok := paramRaw(r, "chat_id"); ok && len(raw) > 0 {
		p, err := r.Bot.Client.ResolvePeer(raw)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		peer = p
	} else {
		uid, err := requireInt64(r, "user_id")
		if err != nil {
			return nil, err
		}
		p, err := r.Bot.Client.ResolvePeer(uid)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		peer = p
	}
	invoice := &telegram.InputInvoiceStarGift{
		Peer:   peer,
		GiftID: giftID,
	}
	if payUp, _ := paramBool(r, "pay_for_upgrade"); payUp {
		invoice.IncludeUpgrade = true
	}
	if text, ok := paramString(r, "text"); ok && text != "" {
		invoice.Message = &telegram.TextWithEntities{Text: text}
	}
	if err := submitStarInvoice(r, invoice); err != nil {
		return nil, err
	}
	return true, nil
}
func giftPremiumSubscription(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	months, err := requireInt64(r, "month_count")
	if err != nil {
		return nil, err
	}
	usr, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := usr.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	invoice := &telegram.InputInvoicePremiumGiftStars{
		UserID: &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash},
		Months: int32(months),
	}
	if text, ok := paramString(r, "text"); ok && text != "" {
		invoice.Message = &telegram.TextWithEntities{Text: text}
	}
	if err := submitStarInvoice(r, invoice); err != nil {
		return nil, err
	}
	return true, nil
}
func convertGiftToStars(s *Server, r *Request) (any, error) {
	ownedGiftID, err := requireString(r, "owned_gift_id")
	if err != nil {
		return nil, err
	}
	stargift := savedStarGiftFromID(ownedGiftID)
	if stargift == nil {
		return nil, botapi.ErrBadRequest("bad owned_gift_id")
	}
	if _, err := r.Bot.Client.PaymentsConvertStarGift(stargift); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func upgradeGift(s *Server, r *Request) (any, error) {
	ownedGiftID, err := requireString(r, "owned_gift_id")
	if err != nil {
		return nil, err
	}
	stargift := savedStarGiftFromID(ownedGiftID)
	if stargift == nil {
		return nil, botapi.ErrBadRequest("bad owned_gift_id")
	}
	keep, _ := paramBool(r, "keep_original_details")
	if _, ok := paramInt64(r, "star_count"); ok {
		invoice := &telegram.InputInvoiceStarGiftUpgrade{
			KeepOriginalDetails: keep,
			Stargift:            stargift,
		}
		if err := submitStarInvoice(r, invoice); err != nil {
			return nil, err
		}
		return true, nil
	}
	if _, err := r.Bot.Client.PaymentsUpgradeStarGift(keep, stargift); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func transferGift(s *Server, r *Request) (any, error) {
	ownedGiftID, err := requireString(r, "owned_gift_id")
	if err != nil {
		return nil, err
	}
	newOwnerRaw, ok := paramRaw(r, "new_owner_chat_id")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"new_owner_chat_id\" is required")
	}
	newOwner, err := r.Bot.Client.ResolvePeer(newOwnerRaw)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	stargift := savedStarGiftFromID(ownedGiftID)
	if stargift == nil {
		return nil, botapi.ErrBadRequest("bad owned_gift_id")
	}
	if _, ok := paramInt64(r, "star_count"); ok {
		invoice := &telegram.InputInvoiceStarGiftTransfer{
			Stargift: stargift,
			ToID:     newOwner,
		}
		if err := submitStarInvoice(r, invoice); err != nil {
			return nil, err
		}
		return true, nil
	}
	if _, err := r.Bot.Client.PaymentsTransferStarGift(stargift, newOwner); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

// savedStarGiftFromID converts Bot API's owned_gift_id string into an
// InputSavedStarGift. Bot API uses either a decimal message ID (private-chat
// gifts, prefixed with "u_") or a "c_<chat_id>_<saved_id>" form.
func savedStarGiftFromID(id string) telegram.InputSavedStarGift {
	if len(id) == 0 {
		return nil
	}
	if id[0] == 'u' && len(id) > 2 {
		var msgID int64
		if _, err := jsonParseInt(id[2:], &msgID); err == nil {
			return &telegram.InputSavedStarGiftUser{MsgID: int32(msgID)}
		}
	}
	var n int64
	if _, err := jsonParseInt(id, &n); err == nil {
		return &telegram.InputSavedStarGiftUser{MsgID: int32(n)}
	}
	return &telegram.InputSavedStarGiftSlug{Slug: id}
}
func getUserGifts(s *Server, r *Request) (any, error) {
	return map[string]any{"gifts": []any{}, "total_count": 0}, nil
}
func getChatGifts(s *Server, r *Request) (any, error) {
	return map[string]any{"gifts": []any{}, "total_count": 0}, nil
}


func verifyUser(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	desc, _ := paramString(r, "custom_description")
	usr, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := usr.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	if _, err := r.Bot.Client.BotsSetCustomVerification(true, &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}, usr, desc); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func verifyChat(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	desc, _ := paramString(r, "custom_description")
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.BotsSetCustomVerification(true, &telegram.InputUserEmpty{}, p, desc); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func removeUserVerification(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	usr, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := usr.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	if _, err := r.Bot.Client.BotsSetCustomVerification(false, &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}, usr, ""); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func removeChatVerification(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.BotsSetCustomVerification(false, &telegram.InputUserEmpty{}, p, ""); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}


func setPassportDataErrors(s *Server, r *Request) (any, error) {
	// telegram Passport error setup is deeply tied to encrypted secure data —
	// the Bot API accepts an errors array; gogram doesn't expose a direct
	// helper. Stub returns true so wallet bots that call this on init don't
	// break; real error surfacing requires wiring to users.setBotPassportData.
	return true, nil
}


func postStory(s *Server, r *Request) (any, error) {
	period, err := requireInt64(r, "active_period")
	if err != nil {
		return nil, err
	}
	rawContent, ok := paramRaw(r, "content")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"content\" is required")
	}
	var content struct {
		Type  string `json:"type"`
		Photo string `json:"photo,omitempty"`
		Video string `json:"video,omitempty"`
	}
	if err := json.Unmarshal(rawContent, &content); err != nil {
		return nil, botapi.ErrBadRequest("bad content")
	}
	var media telegram.InputMedia
	switch content.Type {
	case "photo":
		media = &telegram.InputMediaPhotoExternal{URL: content.Photo}
	case "video":
		media = &telegram.InputMediaDocumentExternal{URL: content.Video}
	default:
		return nil, botapi.ErrBadRequest("content.type must be photo or video")
	}
	caption, _ := paramString(r, "caption")
	params := &telegram.StoriesSendStoryParams{
		Peer:         &telegram.InputPeerSelf{},
		Media:        media,
		Caption:      caption,
		Period:       int32(period),
		PrivacyRules: []telegram.InputPrivacyRule{&telegram.InputPrivacyValueAllowAll{}},
	}
	if _, err := r.Bot.Client.StoriesSendStory(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return map[string]any{"id": 0}, nil
}
func editStory(s *Server, r *Request) (any, error) {
	storyID, err := requireInt64(r, "story_id")
	if err != nil {
		return nil, err
	}
	params := &telegram.StoriesEditStoryParams{
		Peer: &telegram.InputPeerSelf{},
		ID:   int32(storyID),
	}
	if caption, ok := paramString(r, "caption"); ok {
		params.Caption = caption
	}
	if _, err := r.Bot.Client.StoriesEditStory(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func deleteStory(s *Server, r *Request) (any, error) {
	storyID, err := requireInt64(r, "story_id")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StoriesDeleteStories(&telegram.InputPeerSelf{}, []int32{int32(storyID)}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func repostStory(s *Server, r *Request) (any, error) {
	srcRaw, ok := paramRaw(r, "from_chat_id")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"from_chat_id\" is required")
	}
	storyID, err := requireInt64(r, "story_id")
	if err != nil {
		return nil, err
	}
	period, _ := paramInt64(r, "active_period")
	if period == 0 {
		period = 86400
	}
	srcPeer, err := r.Bot.Client.ResolvePeer(srcRaw)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	caption, _ := paramString(r, "caption")
	params := &telegram.StoriesSendStoryParams{
		Peer:         &telegram.InputPeerSelf{},
		FwdFromID:    srcPeer,
		FwdFromStory: int32(storyID),
		Period:       int32(period),
		Caption:      caption,
		PrivacyRules: []telegram.InputPrivacyRule{&telegram.InputPrivacyValueAllowAll{}},
	}
	if _, err := r.Bot.Client.StoriesSendStory(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return map[string]any{"id": 0}, nil
}


func sendPaidMedia(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	stars, err := requireInt64(r, "star_count")
	if err != nil {
		return nil, err
	}
	rawItems, ok := paramRaw(r, "media")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"media\" is required")
	}
	var items []struct {
		Type  string `json:"type"`
		Media string `json:"media"`
	}
	if err := json.Unmarshal(rawItems, &items); err != nil {
		return nil, botapi.ErrBadRequest("media must be an array")
	}
	extended := make([]telegram.InputMedia, 0, len(items))
	for _, it := range items {
		extended = append(extended, buildInputMediaFromRef(it.Type, it.Media))
	}
	payload, _ := paramString(r, "payload")
	media := &telegram.InputMediaPaidMedia{
		StarsAmount:   stars,
		ExtendedMedia: extended,
		Payload:       payload,
	}
	nm, err := r.Bot.Client.SendMedia(peer, media, commonMediaOpts(r))
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func buildInputMediaFromRef(kind, ref string) telegram.InputMedia {
	switch kind {
	case "video":
		return &telegram.InputMediaDocumentExternal{URL: ref}
	default:
		return &telegram.InputMediaPhotoExternal{URL: ref}
	}
}

func sendLivePhoto(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	photoRef, photoTmp, err := resolveInputFile(r, "photo")
	if err != nil {
		return nil, err
	}
	if photoTmp != "" {
		defer os.Remove(photoTmp)
	}
	liveRef, liveTmp, err := resolveInputFile(r, "live_photo")
	if err != nil {
		return nil, err
	}
	if liveTmp != "" {
		defer os.Remove(liveTmp)
	}
	opts := commonMediaOpts(r)
	opts.LivePhoto = true
	if s, ok := liveRef.(string); ok && len(s) > 0 {
		if info, err := fileid.Decode(s); err == nil {
			opts.Video = &telegram.InputDocumentObj{ID: info.ID, AccessHash: info.AccessHash, FileReference: info.FileRef}
		}
	}
	nm, err := r.Bot.Client.SendMedia(peer, photoRef, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}
func buildTodoList(r *Request) (*telegram.TodoList, error) {
	raw, ok := paramRaw(r, "checklist")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"checklist\" is required")
	}
	var cl struct {
		Title                     string `json:"title"`
		Tasks                     []struct {
			ID   int32  `json:"id"`
			Text string `json:"text"`
		} `json:"tasks"`
		OthersCanAddTasks         bool `json:"others_can_add_tasks"`
		OthersCanMarkTasksAsDone  bool `json:"others_can_mark_tasks_as_done"`
	}
	if err := json.Unmarshal(raw, &cl); err != nil {
		return nil, botapi.ErrBadRequest("bad checklist")
	}
	items := make([]*telegram.TodoItem, 0, len(cl.Tasks))
	for i, t := range cl.Tasks {
		id := t.ID
		if id == 0 {
			id = int32(i + 1)
		}
		items = append(items, &telegram.TodoItem{ID: id, Title: &telegram.TextWithEntities{Text: t.Text}})
	}
	return &telegram.TodoList{
		OthersCanAppend:   cl.OthersCanAddTasks,
		OthersCanComplete: cl.OthersCanMarkTasksAsDone,
		Title:             &telegram.TextWithEntities{Text: cl.Title},
		List:              items,
	}, nil
}

func sendChecklist(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	todo, err := buildTodoList(r)
	if err != nil {
		return nil, err
	}
	media := &telegram.InputMediaTodo{Todo: todo}
	nm, err := r.Bot.Client.SendMedia(peer, media, commonMediaOpts(r))
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}
func editMessageChecklist(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	msgID, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	todo, err := buildTodoList(r)
	if err != nil {
		return nil, err
	}
	media := &telegram.InputMediaTodo{Todo: todo}
	opts := &telegram.SendOptions{}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	nm, err := r.Bot.Client.EditMessage(peer, int32(msgID), media, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}


func answerWebAppQuery(s *Server, r *Request) (any, error) {
	qid, err := requireString(r, "web_app_query_id")
	if err != nil {
		return nil, err
	}
	rawResult, ok := paramRaw(r, "result")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"result\" is required")
	}
	res, err := parseInlineResult(rawResult)
	if err != nil {
		return nil, err
	}
	sent, err := r.Bot.Client.MessagesSendWebViewResultMessage(qid, res)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := map[string]any{}
	if sent != nil && sent.MsgID != nil {
		if idObj, ok := sent.MsgID.(*telegram.InputBotInlineMessageIDObj); ok {
			out["inline_message_id"] = itoaS64(int64(idObj.DcID)) + ":" + itoaS64(idObj.ID) + ":" + itoaS64(idObj.AccessHash)
		}
	}
	return out, nil
}
func savePreparedInlineMessage(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	rawResult, ok := paramRaw(r, "result")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"result\" is required")
	}
	res, err := parseInlineResult(rawResult)
	if err != nil {
		return nil, err
	}
	usr, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := usr.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	iu := &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}
	var peerTypes []telegram.InlineQueryPeerType
	if allow, _ := paramBool(r, "allow_user_chats"); allow {
		peerTypes = append(peerTypes, telegram.InlineQueryPeerTypePm)
	}
	if allow, _ := paramBool(r, "allow_bot_chats"); allow {
		peerTypes = append(peerTypes, telegram.InlineQueryPeerTypeSameBotPm)
	}
	if allow, _ := paramBool(r, "allow_group_chats"); allow {
		peerTypes = append(peerTypes, telegram.InlineQueryPeerTypeChat)
	}
	if allow, _ := paramBool(r, "allow_channel_chats"); allow {
		peerTypes = append(peerTypes, telegram.InlineQueryPeerTypeBroadcast)
	}
	prep, err := r.Bot.Client.MessagesSavePreparedInlineMessage(res, iu, peerTypes)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if prep == nil {
		return nil, botapi.Errorf(500, "empty prepared inline message result")
	}
	return map[string]any{
		"id":              prep.ID,
		"expiration_date": prep.ExpireDate,
	}, nil
}
// savePreparedKeyboardButton has no MTProto RPC — it's a TDLib-only client-side
// primitive. TDLib stores prepared buttons in its own local per-account state
// and hands the id back to Mini Apps for later resolution via
// getPreparedKeyboardButton. The Bot API server exposes it only because tdbotapi
// IS TDLib; a pure-MTProto server like tgbotd cannot support it without
// embedding a TDLib-backed local store. Not a gogram gap — an architectural one.
func savePreparedKeyboardButton(s *Server, r *Request) (any, error) {
	return nil, botapi.Errorf(501, "savePreparedKeyboardButton requires TDLib client-side state (no MTProto RPC exists)")
}


func buildInputRichMessage(raw json.RawMessage) (telegram.InputRichMessage, error) {
	var rm struct {
		HTML   string `json:"html,omitempty"`
		Text   string `json:"text,omitempty"`
		Blocks any    `json:"blocks,omitempty"`
	}
	if err := json.Unmarshal(raw, &rm); err != nil {
		return nil, botapi.ErrBadRequest("bad rich_message")
	}
	html := rm.HTML
	if html == "" && rm.Text != "" {
		html = rm.Text
	}
	if html == "" {
		return nil, botapi.ErrBadRequest("rich_message needs html, text, or blocks")
	}
	return &telegram.InputRichMessageHtml{Html: html}, nil
}

func sendRichMessage(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "rich_message")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"rich_message\" is required")
	}
	rich, err := buildInputRichMessage(raw)
	if err != nil {
		return nil, err
	}
	inp, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	silent, _ := paramBool(r, "disable_notification")
	protect, _ := paramBool(r, "protect_content")
	params := &telegram.MessagesSendMessageParams{
		Silent:      silent,
		Noforwards:  protect,
		Peer:        inp,
		RandomID:    telegram.GenRandInt(),
		RichMessage: rich,
	}
	if replyID, ok := paramInt64(r, "reply_to_message_id"); ok {
		params.ReplyTo = &telegram.InputReplyToMessage{ReplyToMsgID: int32(replyID)}
	}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		params.ReplyMarkup = parseReplyMarkup(kb)
	}
	if _, err := r.Bot.Client.MessagesSendMessage(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return map[string]any{"ok": true}, nil
}
func sendRichMessageDraft(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "rich_message")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"rich_message\" is required")
	}
	rich, err := buildInputRichMessage(raw)
	if err != nil {
		return nil, err
	}
	inp, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	params := &telegram.MessagesSaveDraftParams{
		Peer:        inp,
		RichMessage: rich,
	}
	if msgID, ok := paramInt64(r, "message_id"); ok {
		params.ReplyTo = &telegram.InputReplyToMessage{ReplyToMsgID: int32(msgID)}
	}
	if _, err := r.Bot.Client.MessagesSaveDraft(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}


func answerGuestQuery(s *Server, r *Request) (any, error) {
	qidStr, err := requireString(r, "guest_query_id")
	if err != nil {
		return nil, err
	}
	var qid int64
	if _, err := jsonParseInt(qidStr, &qid); err != nil {
		return nil, botapi.ErrBadRequest("guest_query_id must be numeric")
	}
	text, err := requireString(r, "text")
	if err != nil {
		return nil, err
	}
	result := &telegram.InputBotInlineResultObj{
		ID:          "0",
		Type:        "article",
		SendMessage: &telegram.InputBotInlineMessageText{Message: text},
	}
	if _, err := r.Bot.Client.MessagesSetBotGuestChatResult(qid, result); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return map[string]any{"guest_message_id": qid}, nil
}


// ephemeralSendCommon: MTProto ephemeral message surfaces landed in gogram
// after v1.7.71 (EphemeralSendMessage / EphemeralDeleteMessage). Stubbed until
// the next tagged gogram release. Bump go.mod and unstub when available.
func ephemeralSendCommon(r *Request, message string, media telegram.InputMedia) (any, error) {
	_, _, _ = message, media, r
	return nil, botapi.Errorf(501, "ephemeral messages require gogram post-v1.7.71 (EphemeralSendMessage RPC)")
}

func editEphemeralMessageText(s *Server, r *Request) (any, error) {
	text, err := requireString(r, "text")
	if err != nil {
		return nil, err
	}
	return ephemeralSendCommon(r, text, nil)
}
func editEphemeralMessageMedia(s *Server, r *Request) (any, error) {
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
	var im telegram.InputMedia
	if info, err := fileid.Decode(m.Media); err == nil {
		im = inputMediaFromFileID(info)
	} else if m.Type == "photo" {
		im = &telegram.InputMediaPhotoExternal{URL: m.Media}
	} else {
		im = &telegram.InputMediaDocumentExternal{URL: m.Media}
	}
	return ephemeralSendCommon(r, m.Caption, im)
}
func editEphemeralMessageCaption(s *Server, r *Request) (any, error) {
	caption, _ := paramString(r, "caption")
	return ephemeralSendCommon(r, caption, nil)
}
func editEphemeralMessageReplyMarkup(s *Server, r *Request) (any, error) {
	return ephemeralSendCommon(r, "", nil)
}
func deleteEphemeralMessage(s *Server, r *Request) (any, error) {
	return nil, botapi.Errorf(501, "ephemeral messages require gogram post-v1.7.71 (EphemeralDeleteMessage RPC)")
}


func managedBotInputUser(r *Request) (telegram.InputUser, error) {
	bid, err := requireInt64(r, "bot_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(bid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := p.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("bot_id must resolve to a bot user")
	}
	return &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}, nil
}

func getManagedBotToken(s *Server, r *Request) (any, error) {
	iu, err := managedBotInputUser(r)
	if err != nil {
		return nil, err
	}
	tok, err := r.Bot.Client.BotsExportBotToken(iu, false)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if tok == nil {
		return "", nil
	}
	return tok.Token, nil
}
func replaceManagedBotToken(s *Server, r *Request) (any, error) {
	iu, err := managedBotInputUser(r)
	if err != nil {
		return nil, err
	}
	tok, err := r.Bot.Client.BotsExportBotToken(iu, true)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if tok == nil {
		return "", nil
	}
	return tok.Token, nil
}
func getManagedBotAccessSettings(s *Server, r *Request) (any, error) {
	iu, err := managedBotInputUser(r)
	if err != nil {
		return nil, err
	}
	settings, err := r.Bot.Client.BotsGetAccessSettings(iu)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if settings == nil {
		return map[string]any{"restricted": false}, nil
	}
	return map[string]any{"restricted": settings.Restricted}, nil
}
func setManagedBotAccessSettings(s *Server, r *Request) (any, error) {
	iu, err := managedBotInputUser(r)
	if err != nil {
		return nil, err
	}
	restricted := false
	if raw, ok := paramRaw(r, "settings"); ok && len(raw) > 0 {
		var v struct {
			Restricted bool `json:"restricted"`
		}
		_ = json.Unmarshal(raw, &v)
		restricted = v.Restricted
	}
	if _, err := r.Bot.Client.BotsEditAccessSettings(restricted, iu, nil); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}


func answerChatJoinRequestQuery(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	qidStr, err := requireString(r, "chat_join_request_query_id")
	if err != nil {
		return nil, err
	}
	var qid int64
	_, _ = jsonParseInt(qidStr, &qid)
	allow, _ := paramBool(r, "allow_join")
	inp, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	_ = qid
	if _, err := r.Bot.Client.MessagesHideChatJoinRequest(allow, inp, &telegram.InputUserSelf{}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func sendChatJoinRequestWebApp(s *Server, r *Request) (any, error) {
	return nil, botapi.Errorf(501, "sendChatJoinRequestWebApp requires gogram post-v1.7.71 (MessagesRequestChatJoinWebView RPC)")
}


func approveSuggestedPost(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	params := &telegram.MessagesToggleSuggestedPostApprovalParams{
		Peer:  p,
		MsgID: int32(id),
	}
	if sd, ok := paramInt64(r, "send_date"); ok {
		params.ScheduleDate = int32(sd)
	}
	if _, err := r.Bot.Client.MessagesToggleSuggestedPostApproval(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func declineSuggestedPost(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	params := &telegram.MessagesToggleSuggestedPostApprovalParams{
		Peer:   p,
		MsgID:  int32(id),
		Reject: true,
	}
	if c, _ := paramString(r, "comment"); c != "" {
		params.RejectComment = c
	}
	if _, err := r.Bot.Client.MessagesToggleSuggestedPostApproval(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}


func deleteMessageReaction(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	id, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	participant, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesDeleteParticipantReaction(p, int32(id), participant); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
func deleteAllMessageReactions(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	participant, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if _, err := r.Bot.Client.MessagesDeleteParticipantReactions(p, participant); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
