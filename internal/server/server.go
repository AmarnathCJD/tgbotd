package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/config"
)

var bufPool = sync.Pool{New: func() any { b := make([]byte, 0, 4096); return bytes.NewBuffer(b) }}

type Server struct {
	cfg *config.Config
	mgr *botmgr.Manager
	log *slog.Logger
	mux *http.ServeMux
}

func New(cfg *config.Config, mgr *botmgr.Manager, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, mgr: mgr, log: log, mux: http.NewServeMux()}
	s.mux.HandleFunc("/", s.handle)
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.HTTPAddr,
		Handler:           s.mux,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      65 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("listening", "addr", s.cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

type Request struct {
	Ctx    context.Context
	Bot    *botmgr.Bot
	Method string
	Params map[string]json.RawMessage
	Files  map[string]*multipart.FileHeader
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	if n := len(path); n > 0 && path[n-1] == '/' {
		path = path[:n-1]
	}
	if path == "" || path == "healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":"tgbotd"}`))
		return
	}

	if path == "stats" {
		s.handleStats(w, r)
		return
	}

	if strings.HasPrefix(path, "file/bot") {
		s.handleFileDownload(w, r, path)
		return
	}

	if !strings.HasPrefix(path, "bot") {
		writeError(w, botapi.ErrNotFound)
		return
	}
	rest := path[3:]
	sl := strings.IndexByte(rest, '/')
	if sl <= 0 {
		writeError(w, botapi.ErrBadRequest("no method"))
		return
	}
	token := rest[:sl]
	method := lowerASCII(rest[sl+1:])

	ctx := r.Context()
	bot, err := s.mgr.Get(ctx, token)
	if err != nil {
		writeError(w, err)
		return
	}

	if method == "getme" && len(bot.GetMeCache) > 0 && r.URL.RawQuery == "" && r.ContentLength <= 0 {
		hdrs := w.Header()
		hdrs.Set("Content-Type", "application/json; charset=utf-8")
		hdrs.Set("Content-Length", strconv.Itoa(len(bot.GetMeCache)))
		w.WriteHeader(http.StatusOK)
		w.Write(bot.GetMeCache)
		recordCall(method, 0, nil, 0, len(bot.GetMeCache))
		return
	}

	h, ok := handlers[method]
	if !ok {
		writeError(w, botapi.ErrMethodNotFound(method))
		recordCall(method, 0, botapi.ErrMethodNotFound(method), 0, 0)
		return
	}

	params, files, err := parseParams(r)
	if err != nil {
		writeError(w, botapi.ErrBadRequest(err.Error()))
		return
	}

	req := &Request{
		Ctx:    ctx,
		Bot:    bot,
		Method: method,
		Params: params,
		Files:  files,
	}

	start := time.Now()
	result, err := h(s, req)
	dur := time.Since(start)
	bytesIn := int(r.ContentLength)
	if bytesIn < 0 {
		bytesIn = 0
	}
	if err != nil {
		if ae, ok := err.(*botapi.APIError); ok {
			switch ae.Code {
			case 429:
				metrics.FloodWaits.Add(1)
			case 401:
				metrics.Auth401s.Add(1)
			case 403:
				metrics.Forbidden.Add(1)
			}
		}
		s.log.Debug("method error", "bot", bot.BotID, "method", method, "err", err, "dur_ms", dur.Milliseconds())
		writeError(w, err)
		recordCall(method, dur, err, bytesIn, 0)
		return
	}
	writeOK(w, result)
	recordCall(method, dur, nil, bytesIn, 0)
	if dur > 500*time.Millisecond {
		s.log.Info("slow", "bot", bot.BotID, "method", method, "dur_ms", dur.Milliseconds())
	}
}

// lowerASCII lowercases an ASCII string without allocating when it's already lowercase.
func lowerASCII(s string) string {
	needs := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func parseParams(r *http.Request) (map[string]json.RawMessage, map[string]*multipart.FileHeader, error) {
	out := make(map[string]json.RawMessage, 8)
	for k, v := range r.URL.Query() {
		if len(v) == 0 {
			continue
		}
		out[strings.ToLower(k)] = jsonScalar(v[0])
	}

	if r.Method == http.MethodGet {
		return out, nil, nil
	}

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return out, nil, nil
	}
	if strings.HasPrefix(ct, "application/json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil {
			return nil, nil, err
		}
		if len(body) == 0 {
			return out, nil, nil
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, nil, fmt.Errorf("invalid JSON: %w", err)
		}
		for k, v := range raw {
			out[strings.ToLower(k)] = v
		}
		return out, nil, nil
	}
	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return nil, nil, err
		}
		for k, v := range r.PostForm {
			if len(v) == 0 {
				continue
			}
			out[strings.ToLower(k)] = jsonScalar(v[0])
		}
		return out, nil, nil
	}
	mediaType, _, _ := mime.ParseMediaType(ct)
	if strings.HasPrefix(mediaType, "multipart/") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, nil, err
		}
		files := map[string]*multipart.FileHeader{}
		for k, v := range r.MultipartForm.Value {
			if len(v) == 0 {
				continue
			}
			out[strings.ToLower(k)] = jsonScalar(v[0])
		}
		for k, v := range r.MultipartForm.File {
			if len(v) == 0 {
				continue
			}
			files[strings.ToLower(k)] = v[0]
		}
		return out, files, nil
	}
	return out, nil, nil
}

func jsonScalar(s string) json.RawMessage {
	if s == "true" || s == "false" {
		return json.RawMessage(s)
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return json.RawMessage(strconv.FormatInt(n, 10))
	}
	if looksLikeFloat(s) {
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return json.RawMessage(strconv.FormatFloat(n, 'f', -1, 64))
		}
	}
	trimmed := strings.TrimSpace(s)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var probe any
		if json.Unmarshal([]byte(trimmed), &probe) == nil {
			return json.RawMessage(trimmed)
		}
	}
	b, _ := json.Marshal(s)
	return b
}

// looksLikeFloat returns true only if s is a canonical decimal number
// (optional sign, digits, optional dot, digits). "1e10", "Infinity", "NaN"
// return false so they stay as strings — Bot API usernames like "1e10" must
// not be silently coerced to a numeric value.
func looksLikeFloat(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
	}
	sawDigit := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		sawDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			sawDigit = true
			i++
		}
	}
	return sawDigit && i == len(s)
}

// rawResponse is a handler return type that carries an already-encoded JSON
// envelope. handle() writes the bytes verbatim, skipping json.Marshal.
type rawResponse []byte

func writeOK(w http.ResponseWriter, result any) {
	if raw, ok := result.(rawResponse); ok {
		hdrs := w.Header()
		hdrs.Set("Content-Type", "application/json; charset=utf-8")
		hdrs.Set("Content-Length", strconv.Itoa(len(raw)))
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
		return
	}
	resp, err := botapi.OK(result)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeError(w http.ResponseWriter, err error) {
	resp := botapi.FromError(err)
	status := resp.ErrorCode
	if status < 400 || status > 599 {
		status = 400
	}
	writeJSON(w, status, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := buf.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(out)))
	w.WriteHeader(status)
	w.Write(out)
}
