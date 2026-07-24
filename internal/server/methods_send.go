package server

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

func init() {
	register("sendphoto", sendPhoto)
	register("sendaudio", sendAudio)
	register("senddocument", sendDocument)
	register("sendvideo", sendVideo)
	register("sendanimation", sendAnimation)
	register("sendvoice", sendVoice)
	register("sendvideonote", sendVideoNote)
	register("sendsticker", sendSticker)
	register("sendlocation", sendLocation)
	register("sendvenue", sendVenue)
	register("sendcontact", sendContact)
	register("senddice", sendDice)
	register("sendpoll", sendPoll)
	register("sendmediagroup", sendMediaGroup)
}

// sendMediaField wraps the "one input file field" send-media pattern.
// mediaField is the Bot API param name that carries the InputFile-or-string
// (e.g. "photo", "audio", "document"). Extra attributes (Duration, Performer,
// Title, Width, Height, Length, Voice, RoundMessage, Animated, Sticker) are
// filled by mkAttrs based on the method.
func sendMediaField(r *Request, mediaField string, mkAttrs func(*Request, *telegram.MediaOptions)) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	media, tmp, err := resolveInputFile(r, mediaField)
	if err != nil {
		return nil, err
	}
	if tmp != "" {
		defer os.Remove(tmp)
	}
	opts := commonMediaOpts(r)
	if mkAttrs != nil {
		mkAttrs(r, opts)
	}
	// Thumbnail — most media methods accept a "thumbnail" InputFile.
	if _, ok := r.Params["thumbnail"]; ok {
		if thumb, tmpT, err := resolveInputFile(r, "thumbnail"); err == nil {
			opts.Thumb = thumb
			if tmpT != "" {
				defer os.Remove(tmpT)
			}
		}
	}
	// File name override.
	if fn, ok := paramString(r, "file_name"); ok {
		opts.FileName = fn
	}

	nm, err := r.Bot.Client.SendMedia(peer, media, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func sendPhoto(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "photo", nil)
}

func sendAudio(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "audio", func(r *Request, o *telegram.MediaOptions) {
		attrs := []telegram.DocumentAttribute{}
		a := &telegram.DocumentAttributeAudio{}
		anyField := false
		if d, ok := paramInt64(r, "duration"); ok {
			a.Duration = int32(d)
			anyField = true
		}
		if p, ok := paramString(r, "performer"); ok {
			a.Performer = p
			anyField = true
		}
		if t, ok := paramString(r, "title"); ok {
			a.Title = t
			anyField = true
		}
		if anyField {
			attrs = append(attrs, a)
		}
		o.Attributes = attrs
		o.MimeType = "audio/mpeg"
	})
}

func sendDocument(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "document", func(r *Request, o *telegram.MediaOptions) {
		o.ForceDocument = true
		if disable, _ := paramBool(r, "disable_content_type_detection"); disable {
			// gogram detects by extension; no explicit disable — force as document is closest.
		}
	})
}

func sendVideo(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "video", func(r *Request, o *telegram.MediaOptions) {
		attrs := []telegram.DocumentAttribute{}
		a := &telegram.DocumentAttributeVideo{}
		anyField := false
		if d, ok := paramInt64(r, "duration"); ok {
			a.Duration = float64(d)
			anyField = true
		}
		if w, ok := paramInt64(r, "width"); ok {
			a.W = int32(w)
			anyField = true
		}
		if h, ok := paramInt64(r, "height"); ok {
			a.H = int32(h)
			anyField = true
		}
		if streaming, _ := paramBool(r, "supports_streaming"); streaming {
			a.SupportsStreaming = true
			anyField = true
		}
		if anyField {
			attrs = append(attrs, a)
		}
		o.Attributes = attrs
		o.MimeType = "video/mp4"
	})
}

func sendAnimation(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "animation", func(r *Request, o *telegram.MediaOptions) {
		attrs := []telegram.DocumentAttribute{&telegram.DocumentAttributeAnimated{}}
		v := &telegram.DocumentAttributeVideo{}
		anyField := false
		if d, ok := paramInt64(r, "duration"); ok {
			v.Duration = float64(d)
			anyField = true
		}
		if w, ok := paramInt64(r, "width"); ok {
			v.W = int32(w)
			anyField = true
		}
		if h, ok := paramInt64(r, "height"); ok {
			v.H = int32(h)
			anyField = true
		}
		if anyField {
			attrs = append(attrs, v)
		}
		o.Attributes = attrs
		o.MimeType = "video/mp4"
	})
}

func sendVoice(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "voice", func(r *Request, o *telegram.MediaOptions) {
		a := &telegram.DocumentAttributeAudio{Voice: true}
		if d, ok := paramInt64(r, "duration"); ok {
			a.Duration = int32(d)
		}
		o.Attributes = []telegram.DocumentAttribute{a}
		o.MimeType = "audio/ogg"
	})
}

func sendVideoNote(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "video_note", func(r *Request, o *telegram.MediaOptions) {
		v := &telegram.DocumentAttributeVideo{RoundMessage: true}
		if d, ok := paramInt64(r, "duration"); ok {
			v.Duration = float64(d)
		}
		if length, ok := paramInt64(r, "length"); ok {
			v.W = int32(length)
			v.H = int32(length)
		}
		o.Attributes = []telegram.DocumentAttribute{v}
		o.MimeType = "video/mp4"
	})
}

func sendSticker(s *Server, r *Request) (any, error) {
	return sendMediaField(r, "sticker", func(r *Request, o *telegram.MediaOptions) {
		if emoji, ok := paramString(r, "emoji"); ok {
			_ = emoji // gogram picks emoji from sticker attributes; caller override not surfaced.
		}
	})
}

// sendLocation → messages.sendMedia with InputMediaGeoPoint (live variant if
// live_period is set).
func sendLocation(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	lat, lon, err := requireLatLon(r)
	if err != nil {
		return nil, err
	}
	geo := &telegram.InputGeoPointObj{Lat: lat, Long: lon}
	var media telegram.InputMedia
	if lp, ok := paramInt64(r, "live_period"); ok && lp > 0 {
		lm := &telegram.InputMediaGeoLive{GeoPoint: geo, Period: int32(lp)}
		if h, ok := paramInt64(r, "heading"); ok {
			lm.Heading = int32(h)
		}
		if r, ok := paramInt64(r, "proximity_alert_radius"); ok {
			lm.ProximityNotificationRadius = int32(r)
		}
		media = lm
	} else {
		media = &telegram.InputMediaGeoPoint{GeoPoint: geo}
	}
	opts := commonMediaOpts(r)
	nm, err := r.Bot.Client.SendMedia(peer, media, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func sendVenue(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	lat, lon, err := requireLatLon(r)
	if err != nil {
		return nil, err
	}
	title, err := requireString(r, "title")
	if err != nil {
		return nil, err
	}
	address, err := requireString(r, "address")
	if err != nil {
		return nil, err
	}
	fsID, _ := paramString(r, "foursquare_id")
	fsType, _ := paramString(r, "foursquare_type")
	media := &telegram.InputMediaVenue{
		GeoPoint:  &telegram.InputGeoPointObj{Lat: lat, Long: lon},
		Title:     title,
		Address:   address,
		Provider:  "foursquare",
		VenueID:   fsID,
		VenueType: fsType,
	}
	nm, err := r.Bot.Client.SendMedia(peer, media, commonMediaOpts(r))
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func sendContact(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	phone, err := requireString(r, "phone_number")
	if err != nil {
		return nil, err
	}
	first, err := requireString(r, "first_name")
	if err != nil {
		return nil, err
	}
	last, _ := paramString(r, "last_name")
	vcard, _ := paramString(r, "vcard")
	media := &telegram.InputMediaContact{
		PhoneNumber: phone,
		FirstName:   first,
		LastName:    last,
		Vcard:       vcard,
	}
	nm, err := r.Bot.Client.SendMedia(peer, media, commonMediaOpts(r))
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func sendDice(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	emoji, _ := paramString(r, "emoji")
	// gogram's SendDice picks a valid emoji when empty. Passing the emoji
	// directly through InputMediaDice can trip EMOTICON_INVALID on some
	// server-side validators — the helper does the right thing.
	if emoji == "" {
		emoji = "🎲"
	}
	nm, err := r.Bot.Client.SendDice(peer, emoji)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func sendPoll(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	question, err := requireString(r, "question")
	if err != nil {
		return nil, err
	}
	// options: array of {text: ...} objects OR array of strings (legacy)
	rawOpts, ok := paramRaw(r, "options")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"options\" is required")
	}
	var options []string
	{
		var asStrs []string
		if err := json.Unmarshal(rawOpts, &asStrs); err == nil {
			options = asStrs
		} else {
			var asObjs []struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(rawOpts, &asObjs); err != nil {
				return nil, botapi.ErrBadRequest("options must be an array")
			}
			options = make([]string, len(asObjs))
			for i, o := range asObjs {
				options[i] = o.Text
			}
		}
	}

	po := &telegram.PollOptions{}
	if anon, ok := paramBool(r, "is_anonymous"); ok && !anon {
		po.PublicVoters = true
	}
	if multi, ok := paramBool(r, "allows_multiple_answers"); ok && multi {
		po.MCQ = true
	}
	if t, ok := paramString(r, "type"); ok && t == "quiz" {
		po.IsQuiz = true
	}
	if op, ok := paramInt64(r, "open_period"); ok {
		po.ClosePeriod = int32(op)
	}
	if cd, ok := paramInt64(r, "close_date"); ok {
		po.CloseDate = int32(cd)
	}
	if expl, ok := paramString(r, "explanation"); ok {
		po.Solution = expl
	}
	if raw, ok := paramRaw(r, "correct_option_ids"); ok && len(raw) > 0 {
		var ids []int
		if err := json.Unmarshal(raw, &ids); err != nil {
			return nil, botapi.ErrBadRequest("correct_option_ids must be an array of integers")
		}
		po.CorrectAnswers = ids
	}
	if replyID, ok := paramInt64(r, "reply_to_message_id"); ok {
		po.ReplyID = int32(replyID)
	}
	if threadID, ok := paramInt64(r, "message_thread_id"); ok {
		po.TopicID = int32(threadID)
	}
	if protect, _ := paramBool(r, "protect_content"); protect {
		po.NoForwards = true
	}

	nm, err := r.Bot.Client.SendPoll(peer, question, options, po)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

// sendMediaGroup — accepts an array of {type, media, caption, ...} objects.
// Each entry's `media` can be a file_id, URL, or attach://name.
func sendMediaGroup(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	raw, ok := paramRaw(r, "media")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"media\" is required")
	}
	var entries []struct {
		Type            string          `json:"type"`
		Media           string          `json:"media"`
		Caption         string          `json:"caption,omitempty"`
		ParseMode       string          `json:"parse_mode,omitempty"`
		HasSpoiler      bool            `json:"has_spoiler,omitempty"`
		SupportsStreaming bool          `json:"supports_streaming,omitempty"`
		Duration        int32           `json:"duration,omitempty"`
		Width           int32           `json:"width,omitempty"`
		Height          int32           `json:"height,omitempty"`
		Performer       string          `json:"performer,omitempty"`
		Title           string          `json:"title,omitempty"`
		Thumbnail       json.RawMessage `json:"thumbnail,omitempty"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, botapi.ErrBadRequest("media must be a JSON array")
	}
	if len(entries) < 2 || len(entries) > 10 {
		return nil, botapi.ErrBadRequest("media array must contain 2-10 items")
	}
	// Resolve each media entry into a value SendAlbum accepts.
	// gogram accepts a []string or []any where each element is a file path,
	// URL, or previously uploaded InputFile. For file_ids passed as-is we
	// rely on SendMedia semantics inside SendAlbum.
	items := make([]any, 0, len(entries))
	for _, e := range entries {
		if e.Media == "" {
			return nil, botapi.ErrBadRequest("each media entry needs a media field")
		}
		v := e.Media
		if strings.HasPrefix(v, "attach://") {
			attach := strings.ToLower(v[len("attach://"):])
			if fh, ok := r.Files[attach]; ok {
				res, tmp, err := handleMultipartFile(fh)
				if err != nil {
					return nil, err
				}
				if tmp != "" {
					defer os.Remove(tmp)
				}
				items = append(items, res)
				continue
			}
			return nil, botapi.ErrBadRequest("no multipart part named " + attach)
		}
		items = append(items, v)
	}
	opts := commonMediaOpts(r)
	msgs, err := r.Bot.Client.SendAlbum(peer, items, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	out := make([]*botapi.Message, 0, len(msgs))
	for _, nm := range msgs {
		tctx := r.Bot.BuildTranslateContext()
		if bm := tlate.MessageObjToBotAPICtx(newMessageToObj(nm), tctx); bm != nil {
			out = append(out, bm)
		}
	}
	return out, nil
}

// requireLatLon extracts required latitude/longitude fields.
func requireLatLon(r *Request) (float64, float64, error) {
	lat, ok1 := paramFloat(r, "latitude")
	lon, ok2 := paramFloat(r, "longitude")
	if !ok1 || !ok2 {
		return 0, 0, botapi.ErrBadRequest("latitude and longitude required")
	}
	return lat, lon, nil
}

// paramFloat reads a JSON float from the params.
func paramFloat(r *Request, name string) (float64, bool) {
	raw, ok := r.Params[name]
	if !ok {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	return 0, false
}
