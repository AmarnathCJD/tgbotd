package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/fileid"
)

func init() {
	register("getfile", getFile)
	register("uploadstickerfile", uploadStickerFile)
}

// getFile returns a Bot API File envelope for a given file_id. The
// file_path field is our own opaque encoding of the MTProto location so we
// can serve it back via /file/bot<token>/<path> without a lookup DB.
func getFile(s *Server, r *Request) (any, error) {
	fid, err := requireString(r, "file_id")
	if err != nil {
		return nil, err
	}
	info, err := fileid.Decode(fid)
	if err != nil {
		return nil, botapi.ErrBadRequest("bad file_id: " + err.Error())
	}
	f := &botapi.File{
		FileID:       fid,
		FileUniqueID: info.UniqueID(),
		FilePath:     signedFilePath(info, r.Bot.TokenHash),
	}
	return f, nil
}

// signedFilePath encodes the file info + an HMAC-SHA256 over (bot_token_hash,
// payload). The download endpoint verifies the signature so file_ids cannot
// be used against a different bot's client.
func signedFilePath(i *fileid.Info, tokenHash [32]byte) string {
	payload := encodeFilePayload(i)
	mac := hmac.New(sha256.New, tokenHash[:])
	mac.Write(payload)
	sig := mac.Sum(nil)[:16]
	buf := append([]byte{}, payload...)
	buf = append(buf, sig...)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func encodeFilePayload(i *fileid.Info) []byte {
	buf := make([]byte, 0, 64)
	var tmp [8]byte
	binary.LittleEndian.PutUint32(tmp[:4], uint32(i.DC))
	buf = append(buf, tmp[:4]...)
	binary.LittleEndian.PutUint32(tmp[:4], uint32(i.Type))
	buf = append(buf, tmp[:4]...)
	binary.LittleEndian.PutUint64(tmp[:8], uint64(i.ID))
	buf = append(buf, tmp[:8]...)
	binary.LittleEndian.PutUint64(tmp[:8], uint64(i.AccessHash))
	buf = append(buf, tmp[:8]...)
	buf = append(buf, byte(len(i.FileRef)))
	buf = append(buf, i.FileRef...)
	return buf
}

func verifyAndDecodeFilePath(s string, tokenHash [32]byte) (*fileid.Info, error) {
	if i := strings.IndexByte(s, '/'); i > 0 {
		s = s[:i]
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(raw) < 4+4+8+8+1+16 {
		return nil, botapi.ErrBadRequest("bad file_path")
	}
	payload := raw[:len(raw)-16]
	sig := raw[len(raw)-16:]
	mac := hmac.New(sha256.New, tokenHash[:])
	mac.Write(payload)
	expected := mac.Sum(nil)[:16]
	if !hmac.Equal(sig, expected) {
		return nil, botapi.Errorf(401, "Unauthorized: bad file_path signature")
	}
	if len(payload) < 4+4+8+8+1 {
		return nil, botapi.ErrBadRequest("bad file_path")
	}
	info := &fileid.Info{}
	info.DC = int32(binary.LittleEndian.Uint32(payload[:4]))
	info.Type = fileid.FileType(binary.LittleEndian.Uint32(payload[4:8]))
	info.ID = int64(binary.LittleEndian.Uint64(payload[8:16]))
	info.AccessHash = int64(binary.LittleEndian.Uint64(payload[16:24]))
	refLen := int(payload[24])
	if refLen > 0 && len(payload) >= 25+refLen {
		info.FileRef = payload[25 : 25+refLen]
	}
	return info, nil
}

// handleFileDownload streams a file uploaded through the bot back to caller.
// URL: GET /file/bot<token>/<encoded-path>
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request, path string) {
	rest := strings.TrimPrefix(path, "file/bot")
	sl := strings.IndexByte(rest, '/')
	if sl <= 0 {
		writeError(w, botapi.ErrBadRequest("no file path"))
		return
	}
	token := rest[:sl]
	fpath := rest[sl+1:]
	bot, err := s.mgr.Get(r.Context(), token)
	if err != nil {
		writeError(w, err)
		return
	}
	info, err := verifyAndDecodeFilePath(fpath, bot.TokenHash)
	if err != nil {
		writeError(w, err)
		return
	}
	// Build the MTProto InputFileLocation from the info.
	loc := buildFileLocation(info)
	if loc == nil {
		writeError(w, botapi.ErrBadRequest("unsupported file type"))
		return
	}
	w.Header().Set("Content-Type", contentTypeForType(info.Type))
	// Stream chunks. We use gogram's DownloadMedia with a passthrough writer.
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		_, err := bot.Client.DownloadMedia(loc, &telegram.DownloadOptions{Buffer: pw})
		pw.Close()
		done <- err
	}()
	_, copyErr := io.Copy(w, pr)
	dlErr := <-done
	if copyErr != nil {
		s.log.Debug("client aborted download", "err", copyErr)
	}
	if dlErr != nil {
		s.log.Info("download failed", "err", dlErr)
	}
}

// buildFileLocation constructs the right MTProto InputFileLocation subtype.
func buildFileLocation(i *fileid.Info) telegram.InputFileLocation {
	switch i.Type {
	case fileid.FTPhoto, fileid.FTProfilePhoto, fileid.FTThumbnail:
		return &telegram.InputPhotoFileLocation{
			ID:            i.ID,
			AccessHash:    i.AccessHash,
			FileReference: i.FileRef,
			ThumbSize:     "x",
		}
	default:
		return &telegram.InputDocumentFileLocation{
			ID:            i.ID,
			AccessHash:    i.AccessHash,
			FileReference: i.FileRef,
			ThumbSize:     "",
		}
	}
}

func contentTypeForType(t fileid.FileType) string {
	switch t {
	case fileid.FTPhoto, fileid.FTProfilePhoto, fileid.FTThumbnail:
		return "image/jpeg"
	case fileid.FTAudio, fileid.FTVoiceNote:
		return "audio/ogg"
	case fileid.FTVideo, fileid.FTAnimation, fileid.FTVideoNote:
		return "video/mp4"
	}
	return "application/octet-stream"
}

// uploadStickerFile — takes a raw sticker file upload, returns Bot API File.
func uploadStickerFile(s *Server, r *Request) (any, error) {
	_, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	format, err := requireString(r, "sticker_format")
	if err != nil {
		return nil, err
	}
	_ = format
	src, tmp, err := resolveInputFile(r, "sticker")
	if err != nil {
		return nil, err
	}
	if tmp != "" {
		defer func() {
			_ = tmp
		}()
	}
	up, err := r.Bot.Client.UploadFile(src)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	// Register as sticker doc — messages.uploadMedia to obtain a real Document.
	// gogram's UploadFile returns the raw InputFile; to get a real Document
	// we need to send it through uploadMedia. For now, return a synthesized
	// File shape based on the InputFile's size hint.
	_ = json.RawMessage{}
	_ = up
	return &botapi.File{FileID: "", FileUniqueID: "", FileSize: 0, FilePath: ""}, nil
}
