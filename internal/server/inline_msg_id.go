package server

import (
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
)

// decodeInlineMessageID reverses tlate.inlineMsgID. Our translator emits one
// of two forms:
//
//	"<dc>:<id>:<access_hash>"                    -> InputBotInlineMessageIDObj
//	"<dc>:<owner>:<id>:<access_hash>"            -> InputBotInlineMessageID64
func decodeInlineMessageID(s string) (telegram.InputBotInlineMessageID, error) {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 3:
		dc, err1 := atoi64(parts[0])
		id, err2 := atoi64(parts[1])
		ah, err3 := atoi64(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, botapi.ErrBadRequest("bad inline_message_id")
		}
		return &telegram.InputBotInlineMessageIDObj{
			DcID:       int32(dc),
			ID:         id,
			AccessHash: ah,
		}, nil
	case 4:
		dc, err1 := atoi64(parts[0])
		owner, err2 := atoi64(parts[1])
		id, err3 := atoi64(parts[2])
		ah, err4 := atoi64(parts[3])
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			return nil, botapi.ErrBadRequest("bad inline_message_id")
		}
		return &telegram.InputBotInlineMessageID64{
			DcID:       int32(dc),
			OwnerID:    owner,
			ID:         int32(id),
			AccessHash: ah,
		}, nil
	}
	return nil, botapi.ErrBadRequest("bad inline_message_id format")
}

func atoi64(s string) (int64, error) {
	var n int64
	if _, err := jsonParseInt(s, &n); err != nil {
		return 0, err
	}
	return n, nil
}
