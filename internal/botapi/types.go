package botapi

import "encoding/json"

// ChatID represents the polymorphic chat_id field: either int64 or string
// (username). Wraps a raw JSON value so we can pass it through translation
// without losing the original form.
type ChatID struct {
	raw json.RawMessage
}

func (c *ChatID) UnmarshalJSON(b []byte) error {
	c.raw = append(c.raw[:0], b...)
	return nil
}

func (c ChatID) MarshalJSON() ([]byte, error) {
	if len(c.raw) == 0 {
		return []byte("null"), nil
	}
	return c.raw, nil
}

// AsInt tries to parse the ChatID as int64. Returns ok=false if the raw is
// a string or absent.
func (c ChatID) AsInt() (int64, bool) {
	if len(c.raw) == 0 {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal(c.raw, &n); err == nil {
		return n, true
	}
	return 0, false
}

// AsString returns the string form ("@username" etc). ok=false if raw is int.
func (c ChatID) AsString() (string, bool) {
	if len(c.raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(c.raw, &s); err == nil {
		return s, true
	}
	return "", false
}

func (c ChatID) IsZero() bool { return len(c.raw) == 0 }

// User is Bot API User.
type User struct {
	ID                      int64  `json:"id"`
	IsBot                   bool   `json:"is_bot"`
	FirstName               string `json:"first_name"`
	LastName                string `json:"last_name,omitempty"`
	Username                string `json:"username,omitempty"`
	LanguageCode            string `json:"language_code,omitempty"`
	IsPremium               bool   `json:"is_premium,omitempty"`
	AddedToAttachmentMenu   bool   `json:"added_to_attachment_menu,omitempty"`
	CanJoinGroups           bool   `json:"can_join_groups,omitempty"`
	CanReadAllGroupMessages bool   `json:"can_read_all_group_messages,omitempty"`
	SupportsInlineQueries   bool   `json:"supports_inline_queries,omitempty"`
	SupportsGuestQueries    bool   `json:"supports_guest_queries,omitempty"`
	CanConnectToBusiness    bool   `json:"can_connect_to_business,omitempty"`
	HasMainWebApp           bool   `json:"has_main_web_app,omitempty"`
	CanManageBots           bool   `json:"can_manage_bots,omitempty"`
	SupportsJoinRequestQueries bool `json:"supports_join_request_queries,omitempty"`
}

// Chat is the small "Chat" object used inside Message. See ChatFullInfo for
// the full-info variant returned by getChat.
type Chat struct {
	ID              int64  `json:"id"`
	Type            string `json:"type"`
	Title           string `json:"title,omitempty"`
	Username        string `json:"username,omitempty"`
	FirstName       string `json:"first_name,omitempty"`
	LastName        string `json:"last_name,omitempty"`
	IsForum         bool   `json:"is_forum,omitempty"`
	IsDirectMessages bool  `json:"is_direct_messages,omitempty"`
}

// MessageEntity — inline formatting entity.
type MessageEntity struct {
	Type          string `json:"type"`
	Offset        int    `json:"offset"`
	Length        int    `json:"length"`
	URL           string `json:"url,omitempty"`
	User          *User  `json:"user,omitempty"`
	Language      string `json:"language,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
	UnixTime      int64  `json:"unix_time,omitempty"`
	DateTimeFormat string `json:"date_time_format,omitempty"`
}

// Message — minimal shape (all optional fields are actually optional so we
// can grow this without a schema break).
type Message struct {
	MessageID     int64            `json:"message_id"`
	MessageThreadID int64          `json:"message_thread_id,omitempty"`
	From          *User            `json:"from,omitempty"`
	SenderChat    *Chat            `json:"sender_chat,omitempty"`
	Date          int64            `json:"date"`
	Chat          Chat             `json:"chat"`
	ForwardOrigin json.RawMessage  `json:"forward_origin,omitempty"`
	ReplyToMessage *Message        `json:"reply_to_message,omitempty"`
	EditDate      int64            `json:"edit_date,omitempty"`
	MediaGroupID  string           `json:"media_group_id,omitempty"`
	AuthorSignature string         `json:"author_signature,omitempty"`
	Text          string           `json:"text,omitempty"`
	Entities      []MessageEntity  `json:"entities,omitempty"`
	Caption       string           `json:"caption,omitempty"`
	CaptionEntities []MessageEntity `json:"caption_entities,omitempty"`
	ReplyMarkup   json.RawMessage  `json:"reply_markup,omitempty"`
	// Media payload — grown as needed.
	Photo    []PhotoSize `json:"photo,omitempty"`
	Document *Document   `json:"document,omitempty"`
	Audio    *Audio      `json:"audio,omitempty"`
	Video    *Video      `json:"video,omitempty"`
	Voice    *Voice      `json:"voice,omitempty"`
	Sticker  *Sticker    `json:"sticker,omitempty"`
	Animation *Animation `json:"animation,omitempty"`
	VideoNote *VideoNote `json:"video_note,omitempty"`
	Contact  *Contact    `json:"contact,omitempty"`
	Location *Location   `json:"location,omitempty"`
	Venue    *Venue      `json:"venue,omitempty"`
	Dice     *Dice       `json:"dice,omitempty"`
	Poll     *Poll       `json:"poll,omitempty"`
	// Service messages — extend as needed.
	NewChatMembers []User `json:"new_chat_members,omitempty"`
	LeftChatMember *User  `json:"left_chat_member,omitempty"`
	NewChatTitle   string `json:"new_chat_title,omitempty"`
	NewChatPhoto   []PhotoSize `json:"new_chat_photo,omitempty"`
	GroupChatCreated bool `json:"group_chat_created,omitempty"`
	SupergroupChatCreated bool `json:"supergroup_chat_created,omitempty"`
	ChannelChatCreated bool `json:"channel_chat_created,omitempty"`
	MigrateToChatID   int64 `json:"migrate_to_chat_id,omitempty"`
	MigrateFromChatID int64 `json:"migrate_from_chat_id,omitempty"`
	PinnedMessage     *Message `json:"pinned_message,omitempty"`
}

// MessageID — the compact identifier returned by copyMessage etc.
type MessageID struct {
	MessageID int64 `json:"message_id"`
}

// PhotoSize
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// Document
type Document struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
}

// Audio
type Audio struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Duration     int        `json:"duration"`
	Performer    string     `json:"performer,omitempty"`
	Title        string     `json:"title,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
}

// Video
type Video struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	Duration     int        `json:"duration"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
}

// Voice
type Voice struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// Animation
type Animation struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	Duration     int        `json:"duration"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
}

// VideoNote
type VideoNote struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Length       int        `json:"length"`
	Duration     int        `json:"duration"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
}

// Sticker
type Sticker struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Type         string     `json:"type"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	IsAnimated   bool       `json:"is_animated"`
	IsVideo      bool       `json:"is_video"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	Emoji        string     `json:"emoji,omitempty"`
	SetName      string     `json:"set_name,omitempty"`
	CustomEmojiID string    `json:"custom_emoji_id,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
}

// Contact
type Contact struct {
	PhoneNumber string `json:"phone_number"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
	VCard       string `json:"vcard,omitempty"`
}

// Location
type Location struct {
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	HorizontalAccuracy   float64 `json:"horizontal_accuracy,omitempty"`
	LivePeriod           int     `json:"live_period,omitempty"`
	Heading              int     `json:"heading,omitempty"`
	ProximityAlertRadius int     `json:"proximity_alert_radius,omitempty"`
}

// Venue
type Venue struct {
	Location        Location `json:"location"`
	Title           string   `json:"title"`
	Address         string   `json:"address"`
	FoursquareID    string   `json:"foursquare_id,omitempty"`
	FoursquareType  string   `json:"foursquare_type,omitempty"`
	GooglePlaceID   string   `json:"google_place_id,omitempty"`
	GooglePlaceType string   `json:"google_place_type,omitempty"`
}

// Dice
type Dice struct {
	Emoji string `json:"emoji"`
	Value int    `json:"value"`
}

// Poll & PollOption (basic)
type Poll struct {
	ID                    string       `json:"id"`
	Question              string       `json:"question"`
	Options               []PollOption `json:"options"`
	TotalVoterCount       int          `json:"total_voter_count"`
	IsClosed              bool         `json:"is_closed"`
	IsAnonymous           bool         `json:"is_anonymous"`
	Type                  string       `json:"type"`
	AllowsMultipleAnswers bool         `json:"allows_multiple_answers"`
	CorrectOptionID       int          `json:"correct_option_id,omitempty"`
	Explanation           string       `json:"explanation,omitempty"`
	OpenPeriod            int          `json:"open_period,omitempty"`
	CloseDate             int64        `json:"close_date,omitempty"`
}

type PollOption struct {
	Text       string `json:"text"`
	VoterCount int    `json:"voter_count"`
}

// File — response type for getFile / uploadStickerFile.
type File struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int64  `json:"file_size,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
}

// Update — the top-level envelope for getUpdates / webhooks.
type Update struct {
	UpdateID              int64            `json:"update_id"`
	Message               *Message         `json:"message,omitempty"`
	EditedMessage         *Message         `json:"edited_message,omitempty"`
	ChannelPost           *Message         `json:"channel_post,omitempty"`
	EditedChannelPost     *Message         `json:"edited_channel_post,omitempty"`
	BusinessConnection    json.RawMessage  `json:"business_connection,omitempty"`
	BusinessMessage       *Message         `json:"business_message,omitempty"`
	EditedBusinessMessage *Message         `json:"edited_business_message,omitempty"`
	DeletedBusinessMessages json.RawMessage `json:"deleted_business_messages,omitempty"`
	GuestMessage          *Message         `json:"guest_message,omitempty"`
	MessageReaction       json.RawMessage  `json:"message_reaction,omitempty"`
	MessageReactionCount  json.RawMessage  `json:"message_reaction_count,omitempty"`
	InlineQuery           *InlineQuery     `json:"inline_query,omitempty"`
	ChosenInlineResult    json.RawMessage  `json:"chosen_inline_result,omitempty"`
	CallbackQuery         *CallbackQuery   `json:"callback_query,omitempty"`
	ShippingQuery         json.RawMessage  `json:"shipping_query,omitempty"`
	PreCheckoutQuery      json.RawMessage  `json:"pre_checkout_query,omitempty"`
	PurchasedPaidMedia    json.RawMessage  `json:"purchased_paid_media,omitempty"`
	Poll                  *Poll            `json:"poll,omitempty"`
	PollAnswer            json.RawMessage  `json:"poll_answer,omitempty"`
	MyChatMember          json.RawMessage  `json:"my_chat_member,omitempty"`
	ChatMember            json.RawMessage  `json:"chat_member,omitempty"`
	ChatJoinRequest       json.RawMessage  `json:"chat_join_request,omitempty"`
	ChatBoost             json.RawMessage  `json:"chat_boost,omitempty"`
	RemovedChatBoost      json.RawMessage  `json:"removed_chat_boost,omitempty"`
	ManagedBot            json.RawMessage  `json:"managed_bot,omitempty"`
	Subscription          json.RawMessage  `json:"subscription,omitempty"`
}

// InlineQuery basics
type InlineQuery struct {
	ID       string    `json:"id"`
	From     User      `json:"from"`
	Query    string    `json:"query"`
	Offset   string    `json:"offset"`
	ChatType string    `json:"chat_type,omitempty"`
	Location *Location `json:"location,omitempty"`
}

// CallbackQuery basics
type CallbackQuery struct {
	ID              string   `json:"id"`
	From            User     `json:"from"`
	Message         *Message `json:"message,omitempty"`
	InlineMessageID string   `json:"inline_message_id,omitempty"`
	ChatInstance    string   `json:"chat_instance"`
	Data            string   `json:"data,omitempty"`
	GameShortName   string   `json:"game_short_name,omitempty"`
}

// WebhookInfo
type WebhookInfo struct {
	URL                          string   `json:"url"`
	HasCustomCertificate         bool     `json:"has_custom_certificate"`
	PendingUpdateCount           int      `json:"pending_update_count"`
	IPAddress                    string   `json:"ip_address,omitempty"`
	LastErrorDate                int64    `json:"last_error_date,omitempty"`
	LastErrorMessage             string   `json:"last_error_message,omitempty"`
	LastSynchronizationErrorDate int64    `json:"last_synchronization_error_date,omitempty"`
	MaxConnections               int      `json:"max_connections,omitempty"`
	AllowedUpdates               []string `json:"allowed_updates,omitempty"`
}
