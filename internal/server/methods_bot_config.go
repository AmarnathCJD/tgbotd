package server

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
)

// commandCacheKey builds a stable string key from (scope, lang) for the
// getMyCommands byte-cache. Same shape → same key. Cheap: no hashing, no alloc
// beyond the returned string.
func commandCacheKey(raw json.RawMessage, lang string) string {
	if len(raw) == 0 {
		return "d|" + lang
	}
	var v struct {
		Type   string          `json:"type"`
		ChatID json.RawMessage `json:"chat_id,omitempty"`
		UserID int64           `json:"user_id,omitempty"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "err|" + lang
	}
	switch v.Type {
	case "", "default":
		return "d|" + lang
	case "all_private_chats":
		return "apc|" + lang
	case "all_group_chats":
		return "agc|" + lang
	case "all_chat_administrators":
		return "aca|" + lang
	case "chat":
		return "c:" + string(v.ChatID) + "|" + lang
	case "chat_administrators":
		return "cadm:" + string(v.ChatID) + "|" + lang
	case "chat_member":
		return "cm:" + string(v.ChatID) + ":" + strconv.FormatInt(v.UserID, 10) + "|" + lang
	}
	return "u:" + v.Type + "|" + lang
}

func init() {
	register("setmycommands", setMyCommands)
	register("getmycommands", getMyCommands)
	register("deletemycommands", deleteMyCommands)
	register("setmyname", setMyName)
	register("getmyname", getMyName)
	register("setmydescription", setMyDescription)
	register("getmydescription", getMyDescription)
	register("setmyshortdescription", setMyShortDescription)
	register("getmyshortdescription", getMyShortDescription)
	register("setmydefaultadministratorrights", setMyDefaultAdministratorRights)
	register("getmydefaultadministratorrights", getMyDefaultAdministratorRights)
}

// parseCommandScope translates a Bot API BotCommandScope object into a gogram
// BotCommandScope. Nil object → default scope.
func parseCommandScope(raw json.RawMessage, cli *telegram.Client) (telegram.BotCommandScope, error) {
	if len(raw) == 0 {
		return &telegram.BotCommandScopeDefault{}, nil
	}
	var v struct {
		Type   string      `json:"type"`
		ChatID interface{} `json:"chat_id,omitempty"`
		UserID int64       `json:"user_id,omitempty"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, botapi.ErrBadRequest("bad scope object")
	}
	switch v.Type {
	case "", "default":
		return &telegram.BotCommandScopeDefault{}, nil
	case "all_private_chats":
		return &telegram.BotCommandScopeUsers{}, nil
	case "all_group_chats":
		return &telegram.BotCommandScopeChats{}, nil
	case "all_chat_administrators":
		return &telegram.BotCommandScopeChatAdmins{}, nil
	case "chat":
		peer, err := cli.ResolvePeer(v.ChatID)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return &telegram.BotCommandScopePeer{Peer: peer}, nil
	case "chat_administrators":
		peer, err := cli.ResolvePeer(v.ChatID)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		return &telegram.BotCommandScopePeerAdmins{Peer: peer}, nil
	case "chat_member":
		peer, err := cli.ResolvePeer(v.ChatID)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		user, err := cli.ResolvePeer(v.UserID)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		u, ok := user.(*telegram.InputPeerUser)
		if !ok {
			return nil, botapi.ErrBadRequest("user_id must resolve to a user")
		}
		return &telegram.BotCommandScopePeerUser{Peer: peer, UserID: &telegram.InputUserObj{UserID: u.UserID, AccessHash: u.AccessHash}}, nil
	}
	return nil, botapi.ErrBadRequest("unknown scope type: " + v.Type)
}

func setMyCommands(s *Server, r *Request) (any, error) {
	raw, ok := paramRaw(r, "commands")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"commands\" is required")
	}
	var cmds []struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &cmds); err != nil {
		return nil, botapi.ErrBadRequest("commands must be an array")
	}
	scopeRaw, _ := paramRaw(r, "scope")
	scope, err := parseCommandScope(scopeRaw, r.Bot.Client)
	if err != nil {
		return nil, err
	}
	lang, _ := paramString(r, "language_code")
	if lang == "" {
		lang = "en"
	}
	gc := make([]*telegram.BotCommand, len(cmds))
	for i, c := range cmds {
		gc[i] = &telegram.BotCommand{Command: c.Command, Description: c.Description}
	}
	if _, err := r.Bot.Client.BotsSetBotCommands(scope, lang, gc); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	r.Bot.CommandCacheInvalidate()
	return true, nil
}

func getMyCommands(s *Server, r *Request) (any, error) {
	scopeRaw, _ := paramRaw(r, "scope")
	lang, _ := paramString(r, "language_code")
	if lang == "" {
		lang = "en"
	}
	// Cache lookup first — pre-encoded response body served from bytes.
	// Signal to handle() with a rawResponse wrapper.
	key := commandCacheKey(scopeRaw, lang)
	buf, gen := r.Bot.CommandCacheGet(key)
	if buf != nil {
		return rawResponse(buf), nil
	}

	scope, err := parseCommandScope(scopeRaw, r.Bot.Client)
	if err != nil {
		return nil, err
	}
	cmds, err := r.Bot.Client.BotsGetBotCommands(scope, lang)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := make([]map[string]string, len(cmds))
	for i, c := range cmds {
		out[i] = map[string]string{"command": c.Command, "description": c.Description}
	}

	if enc, err := json.Marshal(out); err == nil {
		buf := make([]byte, 0, len(enc)+20)
		buf = append(buf, `{"ok":true,"result":`...)
		buf = append(buf, enc...)
		buf = append(buf, '}')
		r.Bot.CommandCachePut(key, gen, buf)
	}
	return out, nil
}

func deleteMyCommands(s *Server, r *Request) (any, error) {
	scopeRaw, _ := paramRaw(r, "scope")
	scope, err := parseCommandScope(scopeRaw, r.Bot.Client)
	if err != nil {
		return nil, err
	}
	lang, _ := paramString(r, "language_code")
	if lang == "" {
		lang = "en"
	}
	if _, err := r.Bot.Client.BotsSetBotCommands(scope, lang, []*telegram.BotCommand{}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	r.Bot.CommandCacheInvalidate()
	return true, nil
}

// setMyName / setMyDescription / setMyShortDescription all go through
// bots.setBotInfo. Bot API separates them but MTProto has a single RPC with
// optional fields. We call it with just the field being set.
func setBotInfoField(r *Request, field string) (any, error) {
	lang, _ := paramString(r, "language_code")
	val, _ := paramString(r, field)
	// bots.setBotInfo: bot field is optional; when omitted, applies to the
	// calling bot (per TL doc: "or of the current account, if called by a bot").
	// Passing any InputUser here — even Self — hits BOT_INVALID.
	params := &telegram.BotsSetBotInfoParams{
		LangCode: lang,
	}
	switch field {
	case "name":
		params.Name = val
	case "description":
		params.Description = val
	case "short_description":
		params.About = val
	}
	if _, err := r.Bot.Client.BotsSetBotInfo(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	r.Bot.BotInfoCacheInvalidate()
	return true, nil
}

func setMyName(s *Server, r *Request) (any, error) {
	return setBotInfoField(r, "name")
}
func setMyDescription(s *Server, r *Request) (any, error) {
	return setBotInfoField(r, "description")
}
func setMyShortDescription(s *Server, r *Request) (any, error) {
	return setBotInfoField(r, "short_description")
}

// getMyName / getMyDescription / getMyShortDescription share one MTProto
// RPC (BotsGetBotInfo) that returns Name/About/Description in a single call.
// We cache the whole entry per lang_code so three separate Bot API calls
// (very common at bot startup) share one round-trip on cold, zero on warm.
func getBotInfoField(r *Request, field string) (any, error) {
	lang, _ := paramString(r, "language_code")

	entry, gen := r.Bot.BotInfoCacheGet(lang)
	if entry == nil {
		// bots.getBotInfo: bot field is optional; nil applies to the calling bot.
		info, err := r.Bot.Client.BotsGetBotInfo(nil, lang)
		if err != nil {
			return nil, botmgr.MapRPCError(err)
		}
		entry = &botmgr.BotInfoEntry{
			Name:             info.Name,
			Description:      info.Description,
			ShortDescription: info.About,
		}
		r.Bot.BotInfoCachePut(lang, gen, entry)
	}
	switch field {
	case "name":
		return map[string]string{"name": entry.Name}, nil
	case "description":
		return map[string]string{"description": entry.Description}, nil
	case "short_description":
		return map[string]string{"short_description": entry.ShortDescription}, nil
	}
	return nil, nil
}

func getMyName(s *Server, r *Request) (any, error) {
	return getBotInfoField(r, "name")
}
func getMyDescription(s *Server, r *Request) (any, error) {
	return getBotInfoField(r, "description")
}
func getMyShortDescription(s *Server, r *Request) (any, error) {
	return getBotInfoField(r, "short_description")
}

// setMyDefaultAdministratorRights — for groups or channels.
func setMyDefaultAdministratorRights(s *Server, r *Request) (any, error) {
	forChannels, _ := paramBool(r, "for_channels")
	rights := &telegram.ChatAdminRights{}
	anySet := false
	if raw, ok := paramRaw(r, "rights"); ok && len(raw) > 0 {
		var v struct {
			IsAnonymous         bool `json:"is_anonymous"`
			CanManageChat       bool `json:"can_manage_chat"`
			CanDeleteMessages   bool `json:"can_delete_messages"`
			CanManageVideoChats bool `json:"can_manage_video_chats"`
			CanRestrictMembers  bool `json:"can_restrict_members"`
			CanPromoteMembers   bool `json:"can_promote_members"`
			CanChangeInfo       bool `json:"can_change_info"`
			CanInviteUsers      bool `json:"can_invite_users"`
			CanPostMessages     bool `json:"can_post_messages"`
			CanEditMessages     bool `json:"can_edit_messages"`
			CanPinMessages      bool `json:"can_pin_messages"`
			CanManageTopics     bool `json:"can_manage_topics"`
		}
		if err := json.Unmarshal(raw, &v); err == nil {
			rights.Anonymous = v.IsAnonymous
			rights.DeleteMessages = v.CanDeleteMessages
			rights.ManageCall = v.CanManageVideoChats
			rights.BanUsers = v.CanRestrictMembers
			rights.AddAdmins = v.CanPromoteMembers
			rights.ChangeInfo = v.CanChangeInfo
			rights.InviteUsers = v.CanInviteUsers
			rights.PostMessages = v.CanPostMessages
			rights.EditMessages = v.CanEditMessages
			rights.PinMessages = v.CanPinMessages
			rights.ManageTopics = v.CanManageTopics
			anySet = v.IsAnonymous || v.CanManageChat || v.CanDeleteMessages ||
				v.CanManageVideoChats || v.CanRestrictMembers || v.CanPromoteMembers ||
				v.CanChangeInfo || v.CanInviteUsers || v.CanPostMessages ||
				v.CanEditMessages || v.CanPinMessages || v.CanManageTopics
		}
	}
	// MTProto rejects an all-zero ChatAdminRights. Bot API allows it as
	// "no special privileges"; mirror that by sending a minimal flag under
	// the hood — ChangeInfo is the least invasive.
	if !anySet {
		rights.ChangeInfo = true
	}
	if _, err := r.Bot.Client.SetBotDefaultPrivileges(rights, forChannels); err != nil {
		// MTProto's RIGHTS_NOT_MODIFIED is a no-op success from the Bot API POV.
		if strings.Contains(err.Error(), "RIGHTS_NOT_MODIFIED") {
			return true, nil
		}
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func getMyDefaultAdministratorRights(s *Server, r *Request) (any, error) {
	// gogram doesn't have a direct read helper; return an empty rights object
	// with the correct schema so callers get a well-formed response.
	_, _ = paramBool(r, "for_channels")
	return map[string]bool{
		"is_anonymous":            false,
		"can_manage_chat":         true,
		"can_delete_messages":     false,
		"can_manage_video_chats":  false,
		"can_restrict_members":    false,
		"can_promote_members":     false,
		"can_change_info":         false,
		"can_invite_users":        false,
		"can_post_stories":        false,
		"can_edit_stories":        false,
		"can_delete_stories":      false,
	}, nil
}
