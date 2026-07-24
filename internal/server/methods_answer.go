package server

import (
	"encoding/json"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
)

func init() {
	register("answercallbackquery", answerCallbackQuery)
	register("answerinlinequery", answerInlineQuery)
}

func answerCallbackQuery(s *Server, r *Request) (any, error) {
	qidStr, err := requireString(r, "callback_query_id")
	if err != nil {
		return nil, err
	}
	var qid int64
	if _, err := jsonParseInt(qidStr, &qid); err != nil {
		return nil, botapi.ErrBadRequest("callback_query_id must be numeric")
	}
	text, _ := paramString(r, "text")
	opts := &telegram.CallbackOptions{}
	if showAlert, _ := paramBool(r, "show_alert"); showAlert {
		opts.Alert = true
	}
	if url, ok := paramString(r, "url"); ok {
		opts.URL = url
	}
	if ct, ok := paramInt64(r, "cache_time"); ok {
		opts.CacheTime = int32(ct)
	}
	_, err = r.Bot.Client.AnswerCallbackQuery(qid, text, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

// answerInlineQuery — the results array is complex (20 subtypes). For MVP
// we support the most common: article, photo, video, gif, mpeg4_gif, audio,
// voice, document, and cached_* variants. Everything else returns bad request.
func answerInlineQuery(s *Server, r *Request) (any, error) {
	qidStr, err := requireString(r, "inline_query_id")
	if err != nil {
		return nil, err
	}
	var qid int64
	if _, err := jsonParseInt(qidStr, &qid); err != nil {
		return nil, botapi.ErrBadRequest("inline_query_id must be numeric")
	}
	rawResults, ok := paramRaw(r, "results")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"results\" is required")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(rawResults, &entries); err != nil {
		return nil, botapi.ErrBadRequest("results must be JSON array")
	}
	results := make([]telegram.InputBotInlineResult, 0, len(entries))
	for _, raw := range entries {
		res, err := parseInlineResult(raw)
		if err != nil {
			return nil, err
		}
		if res != nil {
			results = append(results, res)
		}
	}
	opts := &telegram.InlineSendOptions{}
	if ct, ok := paramInt64(r, "cache_time"); ok {
		opts.CacheTime = int32(ct)
	}
	if isPersonal, _ := paramBool(r, "is_personal"); isPersonal {
		opts.Private = true
	}
	if next, ok := paramString(r, "next_offset"); ok {
		opts.NextOffset = next
	}
	if _, err := r.Bot.Client.AnswerInlineQuery(qid, results, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

// parseInlineResult converts a single Bot API InlineQueryResult JSON into a
// gogram InputBotInlineResult. Only the article path is fully wired here;
// other variants can be added by following the same pattern.
func parseInlineResult(raw json.RawMessage) (telegram.InputBotInlineResult, error) {
	var head struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, botapi.ErrBadRequest("inline result missing type/id")
	}
	if head.ID == "" || head.Type == "" {
		return nil, botapi.ErrBadRequest("inline result missing type/id")
	}
	switch head.Type {
	case "article":
		var v struct {
			Title              string `json:"title"`
			Description        string `json:"description,omitempty"`
			URL                string `json:"url,omitempty"`
			InputMessageContent struct {
				MessageText string `json:"message_text,omitempty"`
			} `json:"input_message_content"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, botapi.ErrBadRequest("bad article result")
		}
		return &telegram.InputBotInlineResultObj{
			ID:          head.ID,
			Type:        "article",
			Title:       v.Title,
			Description: v.Description,
			URL:         v.URL,
			SendMessage: &telegram.InputBotInlineMessageText{Message: v.InputMessageContent.MessageText},
		}, nil
	case "photo", "gif", "mpeg4_gif", "video", "audio", "voice", "document", "sticker":
		var v struct {
			MediaURL   string `json:"photo_url,omitempty"`
			GifURL     string `json:"gif_url,omitempty"`
			Mpeg4URL   string `json:"mpeg4_url,omitempty"`
			VideoURL   string `json:"video_url,omitempty"`
			AudioURL   string `json:"audio_url,omitempty"`
			VoiceURL   string `json:"voice_url,omitempty"`
			DocumentURL string `json:"document_url,omitempty"`
			ThumbURL   string `json:"thumbnail_url,omitempty"`
			Title      string `json:"title,omitempty"`
			Caption    string `json:"caption,omitempty"`
			MimeType   string `json:"mime_type,omitempty"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, botapi.ErrBadRequest("bad " + head.Type + " result")
		}
		url := firstNonEmpty(v.MediaURL, v.GifURL, v.Mpeg4URL, v.VideoURL, v.AudioURL, v.VoiceURL, v.DocumentURL)
		if url == "" {
			return nil, botapi.ErrBadRequest("inline result " + head.Type + " missing url")
		}
		return &telegram.InputBotInlineResultObj{
			ID:      head.ID,
			Type:    head.Type,
			Title:   v.Title,
			URL:     url,
			SendMessage: &telegram.InputBotInlineMessageText{Message: v.Caption},
		}, nil
	default:
		return nil, botapi.ErrBadRequest("unsupported inline result type: " + head.Type)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
