package server

import (
	"encoding/json"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
)

// Handler is the signature of every Bot API method implementation.
// Return (result, nil) on success; (nil, *botapi.APIError) on failure.
type Handler func(s *Server, r *Request) (any, error)

// handlers is the method name -> handler dispatch map. Keys are lowercase.
var handlers = map[string]Handler{}

// register adds a handler under one or more aliases (all keys lowercase).
func register(name string, h Handler) {
	handlers[name] = h
}

// paramString reads a JSON string value from the request params.
func paramString(r *Request, name string) (string, bool) {
	raw, ok := r.Params[name]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	// numeric-as-string tolerance
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return trimFloat(n), true
	}
	return "", false
}

func trimFloat(n float64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// paramInt64 reads a JSON int64 from the request params.
func paramInt64(r *Request, name string) (int64, bool) {
	raw, ok := r.Params[name]
	if !ok {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int64(f), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		var n int64
		if _, err := jsonParseInt(s, &n); err == nil {
			return n, true
		}
	}
	return 0, false
}

func toInt32Safe(v int64) int32 {
	if v > 2147483647 {
		return 2147483647
	}
	if v < -2147483648 {
		return -2147483648
	}
	return int32(v)
}

func paramBool(r *Request, name string) (bool, bool) {
	raw, ok := r.Params[name]
	if !ok {
		return false, false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "true" {
			return true, true
		}
		if s == "false" {
			return false, true
		}
	}
	return false, false
}

func paramRaw(r *Request, name string) (json.RawMessage, bool) {
	raw, ok := r.Params[name]
	return raw, ok
}

func requireString(r *Request, name string) (string, error) {
	v, ok := paramString(r, name)
	if !ok || v == "" {
		return "", botapi.ErrBadRequest("field \"" + name + "\" is required")
	}
	return v, nil
}

func requireInt64(r *Request, name string) (int64, error) {
	v, ok := paramInt64(r, name)
	if !ok {
		return 0, botapi.ErrBadRequest("field \"" + name + "\" is required")
	}
	return v, nil
}

// jsonParseInt parses a string into int64 without importing strconv here.
func jsonParseInt(s string, out *int64) (int, error) {
	var n int64
	neg := false
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		neg = s[i] == '-'
		i++
	}
	start := i
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return i, errShortInt
		}
		n = n*10 + int64(c-'0')
	}
	if i == start {
		return 0, errShortInt
	}
	if neg {
		n = -n
	}
	*out = n
	return i, nil
}

var errShortInt = botapi.Errorf(400, "invalid integer")
