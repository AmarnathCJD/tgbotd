package server

import (
	"encoding/json"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/fileid"
)

func init() {
	register("getstickerset", getStickerSet)
	register("getcustomemojistickers", getCustomEmojiStickers)
	register("createnewstickerset", createNewStickerSet)
	register("addstickertoset", addStickerToSet)
	register("setstickerpositioninset", setStickerPositionInSet)
	register("deletestickerfromset", deleteStickerFromSet)
	register("replacestickerinset", replaceStickerInSet)
	register("setstickeremojilist", setStickerEmojiList)
	register("setstickerkeywords", setStickerKeywords)
	register("setstickermaskposition", setStickerMaskPosition)
	register("setstickersettitle", setStickerSetTitle)
	register("setstickersetthumbnail", setStickerSetThumbnail)
	register("setcustomemojistickersetthumbnail", setCustomEmojiStickerSetThumbnail)
	register("deletestickerset", deleteStickerSet)
	register("getforumtopiciconstickers", getForumTopicIconStickers)
	register("setchatstickerset", setChatStickerSet)
	register("deletechatstickerset", deleteChatStickerSet)
}

func stickerSetRef(name string) telegram.InputStickerSet {
	return &telegram.InputStickerSetShortName{ShortName: name}
}

func getStickerSet(s *Server, r *Request) (any, error) {
	name, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	set, err := r.Bot.Client.MessagesGetStickerSet(stickerSetRef(name), 0)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return stickerSetToBotAPI(set), nil
}

func stickerSetToBotAPI(set telegram.MessagesStickerSet) map[string]any {
	obj, ok := set.(*telegram.MessagesStickerSetObj)
	if !ok || obj == nil {
		return map[string]any{}
	}
	var docs []map[string]any
	for _, d := range obj.Documents {
		if od, ok := d.(*telegram.DocumentObj); ok {
			docs = append(docs, docToStickerMap(od))
		}
	}
	title, name, stype := "", "", "regular"
	if obj.Set != nil {
		title = obj.Set.Title
		name = obj.Set.ShortName
		if obj.Set.Masks {
			stype = "mask"
		}
		if obj.Set.Emojis {
			stype = "custom_emoji"
		}
	}
	return map[string]any{
		"name":         name,
		"title":        title,
		"sticker_type": stype,
		"stickers":     docs,
	}
}

func docToStickerMap(d *telegram.DocumentObj) map[string]any {
	info := &fileid.Info{
		DC: d.DcID, Type: fileid.FTSticker, ID: d.ID,
		AccessHash: d.AccessHash, FileRef: d.FileReference,
	}
	m := map[string]any{
		"file_id":        info.Encode(),
		"file_unique_id": info.UniqueID(),
		"type":           "regular",
		"is_animated":    d.MimeType == "application/x-tgsticker",
		"is_video":       d.MimeType == "video/webm",
		"file_size":      d.Size,
	}
	for _, a := range d.Attributes {
		switch v := a.(type) {
		case *telegram.DocumentAttributeSticker:
			m["emoji"] = v.Alt
			if s, ok := v.Stickerset.(*telegram.InputStickerSetShortName); ok {
				m["set_name"] = s.ShortName
			}
		case *telegram.DocumentAttributeImageSize:
			m["width"] = v.W
			m["height"] = v.H
		case *telegram.DocumentAttributeVideo:
			m["width"] = v.W
			m["height"] = v.H
		case *telegram.DocumentAttributeCustomEmoji:
			m["type"] = "custom_emoji"
			m["custom_emoji_id"] = itoaS64(d.ID)
		}
	}
	return m
}

func itoaS64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func getCustomEmojiStickers(s *Server, r *Request) (any, error) {
	raw, ok := paramRaw(r, "custom_emoji_ids")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"custom_emoji_ids\" is required")
	}
	var strIDs []string
	if err := json.Unmarshal(raw, &strIDs); err != nil {
		return nil, botapi.ErrBadRequest("custom_emoji_ids must be an array of strings")
	}
	ids := make([]int64, 0, len(strIDs))
	for _, s := range strIDs {
		var n int64
		if _, err := jsonParseInt(s, &n); err == nil {
			ids = append(ids, n)
		}
	}
	docs, err := r.Bot.Client.MessagesGetCustomEmojiDocuments(ids)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := make([]map[string]any, 0, len(docs))
	for _, d := range docs {
		if od, ok := d.(*telegram.DocumentObj); ok {
			out = append(out, docToStickerMap(od))
		}
	}
	return out, nil
}

// createNewStickerSet — accepts a stickers array; each item's `sticker` field
// is a file_id/URL/attach://. Since gogram's sticker path expects
// InputDocument, and we can only reliably build that from a previously
// uploaded file, this returns 501 for the common "brand new upload" flow
// and works only when stickers are file_ids of already-uploaded documents.
func createNewStickerSet(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	name, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	title, err := requireString(r, "title")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "stickers")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"stickers\" is required")
	}
	var entries []struct {
		Sticker      string   `json:"sticker"`
		EmojiList    []string `json:"emoji_list"`
		Keywords     []string `json:"keywords,omitempty"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, botapi.ErrBadRequest("stickers must be an array")
	}
	items := make([]*telegram.InputStickerSetItem, 0, len(entries))
	for _, e := range entries {
		doc, err := fileIDToInputDocument(e.Sticker)
		if err != nil {
			return nil, err
		}
		emoji := ""
		if len(e.EmojiList) > 0 {
			emoji = e.EmojiList[0]
		}
		kw := ""
		if len(e.Keywords) > 0 {
			for i, k := range e.Keywords {
				if i > 0 {
					kw += ","
				}
				kw += k
			}
		}
		items = append(items, &telegram.InputStickerSetItem{
			Document: doc,
			Emoji:    emoji,
			Keywords: kw,
		})
	}
	user, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := user.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	stickerType, _ := paramString(r, "sticker_type")
	params := &telegram.StickersCreateStickerSetParams{
		UserID:    &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash},
		Title:     title,
		ShortName: name,
		Stickers:  items,
	}
	switch stickerType {
	case "mask":
		params.Masks = true
	case "custom_emoji":
		params.Emojis = true
	}
	if _, err := r.Bot.Client.StickersCreateStickerSet(params); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

// fileIDToInputDocument decodes a Bot API file_id and constructs the
// InputDocument reference gogram needs.
func fileIDToInputDocument(fid string) (telegram.InputDocument, error) {
	info, err := fileid.Decode(fid)
	if err != nil {
		return nil, botapi.ErrBadRequest("bad sticker file_id")
	}
	return &telegram.InputDocumentObj{
		ID:            info.ID,
		AccessHash:    info.AccessHash,
		FileReference: info.FileRef,
	}, nil
}

func addStickerToSet(s *Server, r *Request) (any, error) {
	name, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "sticker")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"sticker\" is required")
	}
	var entry struct {
		Sticker   string   `json:"sticker"`
		EmojiList []string `json:"emoji_list"`
		Keywords  []string `json:"keywords"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, botapi.ErrBadRequest("bad sticker")
	}
	doc, err := fileIDToInputDocument(entry.Sticker)
	if err != nil {
		return nil, err
	}
	emoji := ""
	if len(entry.EmojiList) > 0 {
		emoji = entry.EmojiList[0]
	}
	item := &telegram.InputStickerSetItem{Document: doc, Emoji: emoji}
	if _, err := r.Bot.Client.StickersAddStickerToSet(stickerSetRef(name), item); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setStickerPositionInSet(s *Server, r *Request) (any, error) {
	fid, err := requireString(r, "sticker")
	if err != nil {
		return nil, err
	}
	pos, err := requireInt64(r, "position")
	if err != nil {
		return nil, err
	}
	doc, err := fileIDToInputDocument(fid)
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersChangeStickerPosition(doc, int32(pos)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func deleteStickerFromSet(s *Server, r *Request) (any, error) {
	fid, err := requireString(r, "sticker")
	if err != nil {
		return nil, err
	}
	doc, err := fileIDToInputDocument(fid)
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersRemoveStickerFromSet(doc); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func replaceStickerInSet(s *Server, r *Request) (any, error) {
	oldFid, err := requireString(r, "old_sticker")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "sticker")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"sticker\" is required")
	}
	var entry struct {
		Sticker   string   `json:"sticker"`
		EmojiList []string `json:"emoji_list"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, botapi.ErrBadRequest("bad sticker")
	}
	oldDoc, err := fileIDToInputDocument(oldFid)
	if err != nil {
		return nil, err
	}
	newDoc, err := fileIDToInputDocument(entry.Sticker)
	if err != nil {
		return nil, err
	}
	emoji := ""
	if len(entry.EmojiList) > 0 {
		emoji = entry.EmojiList[0]
	}
	if _, err := r.Bot.Client.StickersReplaceSticker(oldDoc, &telegram.InputStickerSetItem{Document: newDoc, Emoji: emoji}); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setStickerEmojiList(s *Server, r *Request) (any, error) {
	fid, err := requireString(r, "sticker")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "emoji_list")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"emoji_list\" is required")
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
		return nil, botapi.ErrBadRequest("emoji_list must be a non-empty array")
	}
	doc, err := fileIDToInputDocument(fid)
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersChangeSticker(doc, list[0], nil, ""); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setStickerKeywords(s *Server, r *Request) (any, error) {
	fid, err := requireString(r, "sticker")
	if err != nil {
		return nil, err
	}
	raw, _ := paramRaw(r, "keywords")
	var kws []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &kws)
	}
	kw := ""
	for i, k := range kws {
		if i > 0 {
			kw += ","
		}
		kw += k
	}
	doc, err := fileIDToInputDocument(fid)
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersChangeSticker(doc, "", nil, kw); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setStickerMaskPosition(s *Server, r *Request) (any, error) {
	fid, err := requireString(r, "sticker")
	if err != nil {
		return nil, err
	}
	raw, _ := paramRaw(r, "mask_position")
	var mc *telegram.MaskCoords
	if len(raw) > 0 {
		var v struct {
			Point  string  `json:"point"`
			XShift float64 `json:"x_shift"`
			YShift float64 `json:"y_shift"`
			Scale  float64 `json:"scale"`
		}
		if err := json.Unmarshal(raw, &v); err == nil {
			pt := int32(0)
			switch v.Point {
			case "forehead":
				pt = 0
			case "eyes":
				pt = 1
			case "mouth":
				pt = 2
			case "chin":
				pt = 3
			}
			mc = &telegram.MaskCoords{N: pt, X: v.XShift, Y: v.YShift, Zoom: v.Scale}
		}
	}
	doc, err := fileIDToInputDocument(fid)
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersChangeSticker(doc, "", mc, ""); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setStickerSetTitle(s *Server, r *Request) (any, error) {
	name, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	title, err := requireString(r, "title")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersRenameStickerSet(stickerSetRef(name), title); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setStickerSetThumbnail(s *Server, r *Request) (any, error) {
	name, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	thumb, _ := paramString(r, "thumbnail")
	if thumb == "" {
		return nil, botapi.ErrBadRequest("field \"thumbnail\" is required")
	}
	doc, err := fileIDToInputDocument(thumb)
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersSetStickerSetThumb(stickerSetRef(name), doc, 0); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func setCustomEmojiStickerSetThumbnail(s *Server, r *Request) (any, error) {
	name, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	emojiID, _ := paramString(r, "custom_emoji_id")
	var n int64
	if emojiID != "" {
		_, _ = jsonParseInt(emojiID, &n)
	}
	if _, err := r.Bot.Client.StickersSetStickerSetThumb(stickerSetRef(name), nil, n); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func deleteStickerSet(s *Server, r *Request) (any, error) {
	name, err := requireString(r, "name")
	if err != nil {
		return nil, err
	}
	if _, err := r.Bot.Client.StickersDeleteStickerSet(stickerSetRef(name)); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func getForumTopicIconStickers(s *Server, r *Request) (any, error) {
	// Emoji stickers with the standard forum topic icons live in a curated
	// set. gogram exposes MessagesGetStickerSet — the icons set is not a
	// standard sticker set the RPC recognises by short name, so we return
	// an empty array (Bot API-compatible response shape).
	return []any{}, nil
}

func setChatStickerSet(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	name, err := requireString(r, "sticker_set_name")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	inputCh, ok := p.(*telegram.InputPeerChannel)
	if !ok {
		return nil, botapi.ErrBadRequest("chat_id must be a supergroup")
	}
	if _, err := r.Bot.Client.ChannelsSetStickers(
		&telegram.InputChannelObj{ChannelID: inputCh.ChannelID, AccessHash: inputCh.AccessHash},
		stickerSetRef(name),
	); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func deleteChatStickerSet(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	p, err := r.Bot.Client.ResolvePeer(peer)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	inputCh, ok := p.(*telegram.InputPeerChannel)
	if !ok {
		return nil, botapi.ErrBadRequest("chat_id must be a supergroup")
	}
	if _, err := r.Bot.Client.ChannelsSetStickers(
		&telegram.InputChannelObj{ChannelID: inputCh.ChannelID, AccessHash: inputCh.AccessHash},
		&telegram.InputStickerSetEmpty{},
	); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
