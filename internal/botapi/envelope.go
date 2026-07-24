package botapi

import (
	"encoding/json"
	"fmt"
)

type Response struct {
	OK          bool                `json:"ok"`
	Result      json.RawMessage     `json:"result,omitempty"`
	ErrorCode   int                 `json:"error_code,omitempty"`
	Description string              `json:"description,omitempty"`
	Parameters  *ResponseParameters `json:"parameters,omitempty"`
}

type ResponseParameters struct {
	MigrateToChatID int64 `json:"migrate_to_chat_id,omitempty"`
	RetryAfter      int   `json:"retry_after,omitempty"`
}

type APIError struct {
	Code        int
	Description string
	Parameters  *ResponseParameters
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Description)
}

func Errorf(code int, format string, args ...any) *APIError {
	return &APIError{Code: code, Description: fmt.Sprintf(format, args...)}
}

var (
	ErrUnauthorized = &APIError{Code: 401, Description: "Unauthorized"}
	ErrNotFound     = &APIError{Code: 404, Description: "Not Found"}
)

func ErrMethodNotFound(name string) *APIError {
	return &APIError{Code: 404, Description: fmt.Sprintf("Not Found: method %q not implemented", name)}
}

func ErrBadRequest(desc string) *APIError {
	return &APIError{Code: 400, Description: "Bad Request: " + desc}
}

func ErrInternal(desc string) *APIError {
	return &APIError{Code: 500, Description: "Internal Server Error: " + desc}
}

func FloodWait(seconds int) *APIError {
	return &APIError{
		Code:        429,
		Description: fmt.Sprintf("Too Many Requests: retry after %d", seconds),
		Parameters:  &ResponseParameters{RetryAfter: seconds},
	}
}

func Migrate(newChatID int64) *APIError {
	return &APIError{
		Code:        400,
		Description: "Bad Request: group chat was upgraded to a supergroup chat",
		Parameters:  &ResponseParameters{MigrateToChatID: newChatID},
	}
}

func OK(result any) (*Response, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &Response{OK: true, Result: b}, nil
}

func FromError(err error) *Response {
	if err == nil {
		return &Response{OK: true, Result: json.RawMessage(`true`)}
	}
	if ae, ok := err.(*APIError); ok {
		return &Response{
			OK:          false,
			ErrorCode:   ae.Code,
			Description: ae.Description,
			Parameters:  ae.Parameters,
		}
	}
	return &Response{
		OK:          false,
		ErrorCode:   500,
		Description: "Internal Server Error: " + err.Error(),
	}
}
