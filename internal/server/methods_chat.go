package server

import (
	"encoding/json"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

func init() {
	register("getchat", getChat)
	register("getchatmember", getChatMember)
	register("getchatmembercount", getChatMemberCount)
	register("getchatadministrators", getChatAdministrators)
	register("banchatmember", banChatMember)
	register("unbanchatmember", unbanChatMember)
	register("restrictchatmember", restrictChatMember)
	register("promotechatmember", promoteChatMember)
	register("leavechat", leaveChat)
	register("setchattitle", setChatTitle)
	register("pinchatmessage", pinChatMessage)
	register("unpinchatmessage", unpinChatMessage)
	register("exportchatinvitelink", exportChatInviteLink)
}

// getChat returns a Bot API ChatFullInfo (we return the compact Chat plus
// title/username fields; growable).
func getChat(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	resolved, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	switch v := resolved.(type) {
	case *telegram.InputPeerUser:
		u, err := r.Bot.Client.GetUser(v.UserID)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return &botapi.Chat{
			ID:        u.ID,
			Type:      "private",
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Username:  u.Username,
		}, nil
	case *telegram.InputPeerChat:
		return &botapi.Chat{ID: -v.ChatID, Type: "group"}, nil
	case *telegram.InputPeerChannel:
		ch, err := r.Bot.Client.GetChannel(v.ChannelID)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		chatType := "supergroup"
		if ch.Broadcast {
			chatType = "channel"
		}
		return &botapi.Chat{
			ID:       tlate.BotAPIChatID(ch.ID, true),
			Type:     chatType,
			Title:    ch.Title,
			Username: ch.Username,
			IsForum:  ch.Forum,
		}, nil
	}
	return nil, botapi.ErrBadRequest("could not resolve chat")
}

func getChatMember(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	// For DMs (peer is a user), Bot API returns a synthetic ChatMember: the
	// user is a "member" if they match the chat_id, else 404. MTProto has no
	// participant concept for user peers.
	if ipeer, rerr := r.Bot.Client.ResolvePeer(peer); rerr == nil {
		if pu, ok := ipeer.(*telegram.InputPeerUser); ok {
			if u, uerr := r.Bot.Client.GetUser(uid); uerr == nil {
				me := r.Bot.Client.Me()
				status := "member"
				if me != nil && uid == me.ID {
					status = "administrator"
				}
				if uid != pu.UserID && (me == nil || uid != me.ID) {
					return nil, botapi.Errorf(400, "user is not a member of the private chat")
				}
				return map[string]any{
					"status": status,
					"user":   tlate.UserFromObj(u),
				}, nil
			}
		}
	}
	p, err := r.Bot.Client.GetChatMember(peer, uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return partToBotAPI(p), nil
}

func getChatMemberCount(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	// A DM is a two-participant "chat" from Bot API's POV: the user + the bot.
	if ipeer, rerr := r.Bot.Client.ResolvePeer(peer); rerr == nil {
		if _, ok := ipeer.(*telegram.InputPeerUser); ok {
			return 2, nil
		}
	}
	n, err := r.Bot.Client.GetChatMembersCount(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return int(n), nil
}

func getChatAdministrators(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	participants, _, err := r.Bot.Client.GetChatMembers(peer, &telegram.ParticipantOptions{Filter: &telegram.ChannelParticipantsAdmins{}})
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := make([]map[string]any, 0, len(participants))
	for _, p := range participants {
		out = append(out, partToBotAPI(p))
	}
	return out, nil
}

// partToBotAPI converts gogram Participant → Bot API ChatMember-shaped map.
func partToBotAPI(p *telegram.Participant) map[string]any {
	if p == nil {
		return nil
	}
	m := map[string]any{
		"status": p.Status,
		"user":   tlate.UserFromObj(p.User),
	}
	if p.Rank != "" {
		m["custom_title"] = p.Rank
	}
	if p.Rights != nil {
		if p.Status == "administrator" || p.Status == "creator" {
			m["can_manage_chat"] = p.Rights.ChangeInfo
			m["can_delete_messages"] = p.Rights.DeleteMessages
			m["can_manage_video_chats"] = p.Rights.ManageCall
			m["can_restrict_members"] = p.Rights.BanUsers
			m["can_promote_members"] = p.Rights.AddAdmins
			m["can_change_info"] = p.Rights.ChangeInfo
			m["can_invite_users"] = p.Rights.InviteUsers
			m["can_post_messages"] = p.Rights.PostMessages
			m["can_edit_messages"] = p.Rights.EditMessages
			m["can_pin_messages"] = p.Rights.PinMessages
			m["can_manage_topics"] = p.Rights.ManageTopics
			m["is_anonymous"] = p.Rights.Anonymous
		}
	}
	return m
}

func banChatMember(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.BannedOptions{Ban: true, Rights: &telegram.ChatBannedRights{}}
	if until, ok := paramInt64(r, "until_date"); ok {
		opts.TillDate = int32(until)
	}
	if revoke, _ := paramBool(r, "revoke_messages"); revoke {
		opts.Revoke = true
	}
	if _, err := r.Bot.Client.EditBanned(peer, uid, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func unbanChatMember(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.BannedOptions{Unban: true, Rights: &telegram.ChatBannedRights{}}
	if _, err := r.Bot.Client.EditBanned(peer, uid, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func restrictChatMember(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	rights := &telegram.ChatBannedRights{}
	// permissions object
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
		}
		if err := jsonUnmarshalRaw(raw, &perm); err == nil {
			// Bot API "can_X" bools are the ALLOW form; MTProto uses the BAN form.
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
		}
	}
	opts := &telegram.BannedOptions{Mute: true, Rights: rights}
	if until, ok := paramInt64(r, "until_date"); ok {
		opts.TillDate = int32(until)
	}
	if _, err := r.Bot.Client.EditBanned(peer, uid, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func promoteChatMember(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	rights := &telegram.ChatAdminRights{}
	get := func(name string) bool {
		v, _ := paramBool(r, name)
		return v
	}
	rights.ChangeInfo = get("can_change_info")
	rights.PostMessages = get("can_post_messages")
	rights.EditMessages = get("can_edit_messages")
	rights.DeleteMessages = get("can_delete_messages")
	rights.BanUsers = get("can_restrict_members")
	rights.InviteUsers = get("can_invite_users")
	rights.PinMessages = get("can_pin_messages")
	rights.AddAdmins = get("can_promote_members")
	rights.Anonymous = get("is_anonymous")
	rights.ManageCall = get("can_manage_video_chats")
	rights.ManageTopics = get("can_manage_topics")
	// Bot API "can_manage_chat" is implied when any admin right is granted.

	isAdmin := rights.ChangeInfo || rights.PostMessages || rights.EditMessages ||
		rights.DeleteMessages || rights.BanUsers || rights.InviteUsers ||
		rights.PinMessages || rights.AddAdmins || rights.Anonymous ||
		rights.ManageCall || rights.ManageTopics
	opts := &telegram.AdminOptions{IsAdmin: isAdmin, Rights: rights, Rank: ""}
	if _, err := r.Bot.Client.EditAdmin(peer, uid, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func leaveChat(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	if err := r.Bot.Client.LeaveChannel(peer); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setChatTitle(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	title, err := requireString(r, "title")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.EditTitle(peer, title); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func pinChatMessage(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	msgID, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	opts := &telegram.PinOptions{}
	if silent, _ := paramBool(r, "disable_notification"); silent {
		opts.Silent = true
	}
	if _, err := r.Bot.Client.PinMessage(peer, int32(msgID), opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func unpinChatMessage(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	msgID, err := requireInt64(r, "message_id")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.UnpinMessage(peer, int32(msgID)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func exportChatInviteLink(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	link, err := r.Bot.Client.ExportInvite(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if l, ok := link.(*telegram.ChatInviteExported); ok {
		return l.Link, nil
	}
	return "", nil
}

func jsonUnmarshalRaw(raw []byte, v any) error {
	return json.Unmarshal(raw, v)
}
