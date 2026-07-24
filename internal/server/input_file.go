package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/fileid"
)

const maxURLDownloadBytes = 2 * 1024 * 1024 * 1024

var urlDownloadClient = &http.Client{Timeout: 120 * time.Second}

// downloadURLToTemp fetches an HTTP(S) URL, streams up to maxBytes into a
// temp file, and returns its absolute path. The caller must os.Remove the
// path when done. Matches the official Bot API server's "URL means server
// downloads then re-uploads" contract.
func downloadURLToTemp(ctx context.Context, url string, maxBytes int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := urlDownloadClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	base := path.Base(req.URL.Path)
	if base == "" || base == "/" || base == "." {
		base = "download"
	}
	base = sanitizeFileName(base)
	ext := ""
	if i := strings.LastIndexByte(base, '.'); i > 0 {
		ext = base[i:]
		base = base[:i]
	}
	if ext == "" {
		ext = extFromContentType(resp.Header.Get("Content-Type"))
	}
	f, err := os.CreateTemp("", "tgbotd-url-"+base+"-*"+ext)
	if err != nil {
		return "", err
	}
	limited := io.LimitReader(resp.Body, maxBytes)
	if _, err := io.Copy(f, limited); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func extFromContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i > 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	switch ct {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "audio/wav":
		return ".wav"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	}
	return ""
}

func sanitizeFileName(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.' || c == '-' || c == '_':
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "download"
	}
	return string(out)
}

// inputMediaFromFileID reconstructs the correct MTProto InputMedia for a
// previously-uploaded file, so we send by reference rather than re-uploading.
func inputMediaFromFileID(info *fileid.Info) telegram.InputMedia {
	switch info.Type {
	case fileid.FTPhoto, fileid.FTProfilePhoto, fileid.FTThumbnail:
		return &telegram.InputMediaPhoto{
			ID: &telegram.InputPhotoObj{
				ID:            info.ID,
				AccessHash:    info.AccessHash,
				FileReference: info.FileRef,
			},
		}
	default:
		return &telegram.InputMediaDocument{
			ID: &telegram.InputDocumentObj{
				ID:            info.ID,
				AccessHash:    info.AccessHash,
				FileReference: info.FileRef,
			},
		}
	}
}

// resolveInputFile turns a Bot API "InputFile or String" value from the
// request into something gogram.UploadFile / SendMedia accept.
//
// Accepted forms per Bot API:
//   1. file_id (string) — a previous file_id we can round-trip back to MTProto
//   2. HTTP(S) URL — Telegram fetches directly; gogram accepts as string
//   3. attach://<name> — refers to another multipart part in the same request
//   4. multipart field with the same param name — direct upload
//
// Returns one of:
//   - string (URL or path)
//   - *telegram.InputMediaPhoto / *InputMediaDocument (when file_id resolved)
//   - a temp file path (multipart upload → written to temp, path returned)
func resolveInputFile(r *Request, field string) (any, string, error) {
	// Try string first — file_id or URL.
	if raw, ok := paramRaw(r, field); ok && len(raw) > 0 {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			// attach:// reference — resolve the underlying multipart part.
			if strings.HasPrefix(s, "attach://") {
				attachName := strings.TrimPrefix(s, "attach://")
				if fh, ok := r.Files[strings.ToLower(attachName)]; ok {
					return handleMultipartFile(fh)
				}
				return nil, "", botapi.ErrBadRequest("attach://" + attachName + " has no matching multipart part")
			}
			if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
				ctx := r.Ctx
				if ctx == nil {
					ctx = context.Background()
				}
				path, err := downloadURLToTemp(ctx, s, maxURLDownloadBytes)
				if err != nil {
					return nil, "", botapi.ErrBadRequest("failed to fetch URL: " + err.Error())
				}
				return path, path, nil
			}
			if info, err := fileid.Decode(s); err == nil {
				return inputMediaFromFileID(info), "", nil
			}
			return s, "", nil
		}
	}
	// Direct multipart file with the same field name.
	if fh, ok := r.Files[field]; ok {
		return handleMultipartFile(fh)
	}
	return nil, "", botapi.ErrBadRequest("field \"" + field + "\" is required")
}

// handleMultipartFile writes the uploaded part to a temp file and returns
// its path. gogram's UploadFile accepts a path string; we clean up the temp
// file after the request finishes.
func handleMultipartFile(fh *multipart.FileHeader) (any, string, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, "", botmgr.MapRPCError(err)
	}
	defer f.Close()
	dir := os.TempDir()
	name := filepath.Base(fh.Filename)
	if name == "" || name == "." {
		name = "upload"
	}
	tmp, err := os.CreateTemp(dir, "tgbotd-"+name+"-*")
	if err != nil {
		return nil, "", botmgr.MapRPCError(err)
	}
	if _, err := io.Copy(tmp, f); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, "", botmgr.MapRPCError(err)
	}
	tmp.Close()
	return tmp.Name(), tmp.Name(), nil
}

// commonMediaOpts extracts the send-options that apply to every media-send
// method (silent, protect_content, reply_parameters, reply_markup, thread,
// business_connection_id, message_effect_id, allow_paid_broadcast).
func commonMediaOpts(r *Request) *telegram.MediaOptions {
	opts := &telegram.MediaOptions{}
	if silent, _ := paramBool(r, "disable_notification"); silent {
		opts.Silent = true
	}
	if protect, _ := paramBool(r, "protect_content"); protect {
		opts.NoForwards = true
	}
	if threadID, ok := paramInt64(r, "message_thread_id"); ok {
		opts.TopicID = int32(threadID)
	}
	if replyID, ok := paramInt64(r, "reply_to_message_id"); ok {
		opts.ReplyID = int32(replyID)
	}
	if rp, ok := paramRaw(r, "reply_parameters"); ok && len(rp) > 0 {
		var rpv struct {
			MessageID int32 `json:"message_id"`
		}
		if err := json.Unmarshal(rp, &rpv); err == nil && rpv.MessageID != 0 {
			opts.ReplyID = rpv.MessageID
		}
	}
	if kb, ok := paramRaw(r, "reply_markup"); ok && len(kb) > 0 {
		opts.ReplyMarkup = parseReplyMarkup(kb)
	}
	if caption, ok := paramString(r, "caption"); ok {
		opts.Caption = caption
	}
	if pm, ok := paramString(r, "parse_mode"); ok {
		opts.ParseMode = pm
	}
	if hasSpoiler, _ := paramBool(r, "has_spoiler"); hasSpoiler {
		opts.Spoiler = true
	}
	if showAbove, _ := paramBool(r, "show_caption_above_media"); showAbove {
		opts.InvertMedia = true
	}
	return opts
}
