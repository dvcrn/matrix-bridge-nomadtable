package nomadtable

import "time"

type UpdateUserResponse struct {
	Users                    map[string]*UserResponse `json:"users"`
	MembershipDeletionTaskID string                   `json:"membership_deletion_task_id"`
	Duration                 string                   `json:"duration"`
}

type UserResponse struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Language         string     `json:"language"`
	Role             string     `json:"role"`
	Teams            []any      `json:"teams"`
	CreatedAt        *time.Time `json:"created_at"`
	UpdatedAt        *time.Time `json:"updated_at"`
	Banned           bool       `json:"banned"`
	Online           bool       `json:"online"`
	LastActive       *time.Time `json:"last_active"`
	Devices          []*Device  `json:"devices"`
	Invisible        bool       `json:"invisible"`
	Mutes            []any      `json:"mutes"`
	ChannelMutes     []any      `json:"channel_mutes"`
	UnreadCount      int        `json:"unread_count"`
	TotalUnreadCount int        `json:"total_unread_count"`
	UnreadChannels   int        `json:"unread_channels"`
	UnreadThreads    int        `json:"unread_threads"`
	ShadowBanned     bool       `json:"shadow_banned"`
	BlockedUserIDs   []any      `json:"blocked_user_ids"`
	Gender           string     `json:"gender"`
	ProfileImage     string     `json:"profileImage"`
}

type Device struct {
	PushProvider     string     `json:"push_provider"`
	PushProviderName string     `json:"push_provider_name"`
	ID               string     `json:"id"`
	CreatedAt        *time.Time `json:"created_at"`
	UserID           string     `json:"user_id"`
}

type GetAppResponse struct {
	App      *App   `json:"app"`
	Duration string `json:"duration"`
}

type App struct {
	ID                     int           `json:"id"`
	Name                   string        `json:"name"`
	Placement              string        `json:"placement"`
	AsyncURLEnrichEnabled  bool          `json:"async_url_enrich_enabled"`
	AutoTranslationEnabled bool          `json:"auto_translation_enabled"`
	FileUploadConfig       *UploadConfig `json:"file_upload_config"`
	ImageUploadConfig      *UploadConfig `json:"image_upload_config"`
	VideoProvider          string        `json:"video_provider"`
}

type UploadConfig struct {
	AllowedFileExtensions []string `json:"allowed_file_extensions"`
	BlockedFileExtensions []string `json:"blocked_file_extensions"`
	AllowedMimeTypes      []string `json:"allowed_mime_types"`
	BlockedMimeTypes      []string `json:"blocked_mime_types"`
	SizeLimit             int      `json:"size_limit"`
}

type QueryChannelsResponse struct {
	Channels []ChannelState `json:"channels"`
	Duration string         `json:"duration"`
}

type QueryChannelResponse struct {
	Channel        *Channel         `json:"channel"`
	Messages       []*Message       `json:"messages"`
	PinnedMessages []*Message       `json:"pinned_messages"`
	WatcherCount   int              `json:"watcher_count"`
	Read           []*Read          `json:"read"`
	Members        []*ChannelMember `json:"members"`
	Membership     *ChannelMember   `json:"membership"`
	Threads        []any            `json:"threads"`
	Duration       string           `json:"duration"`
}

type QueryMembersResponse struct {
	Members  []*ChannelMember `json:"members"`
	Duration string           `json:"duration"`
}

type SendMessageResponse struct {
	Message  *Message `json:"message"`
	Duration string   `json:"duration"`
}

type MarkReadResponse struct {
	Event    *MessageReadEvent `json:"event"`
	Duration string            `json:"duration"`
}

type ChannelState struct {
	Channel        *Channel         `json:"channel"`
	Messages       []*Message       `json:"messages"`
	PinnedMessages []*Message       `json:"pinned_messages"`
	WatcherCount   int              `json:"watcher_count"`
	Read           []*Read          `json:"read"`
	Members        []*ChannelMember `json:"members"`
	Membership     *ChannelMember   `json:"membership"`
	Threads        []any            `json:"threads"`
}

type Channel struct {
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	CID             string         `json:"cid"`
	LastMessageAt   *time.Time     `json:"last_message_at"`
	CreatedAt       *time.Time     `json:"created_at"`
	UpdatedAt       *time.Time     `json:"updated_at"`
	CreatedBy       *UserResponse  `json:"created_by"`
	Frozen          bool           `json:"frozen"`
	Disabled        bool           `json:"disabled"`
	MemberCount     int            `json:"member_count"`
	Config          *ChannelConfig `json:"config"`
	OwnCapabilities []string       `json:"own_capabilities"`
	Hidden          bool           `json:"hidden"`
	Blocked         bool           `json:"blocked"`
	Name            string         `json:"name"`
	Image           string         `json:"image,omitempty"`
	PlanID          string         `json:"plan_id,omitempty"`
	ActivityEmoji   string         `json:"activityEmoji,omitempty"`
}

type ChannelConfig struct {
	CreatedAt                      *time.Time `json:"created_at"`
	UpdatedAt                      *time.Time `json:"updated_at"`
	Name                           string     `json:"name"`
	TypingEvents                   bool       `json:"typing_events"`
	ReadEvents                     bool       `json:"read_events"`
	ConnectEvents                  bool       `json:"connect_events"`
	DeliveryEvents                 bool       `json:"delivery_events"`
	Search                         bool       `json:"search"`
	Reactions                      bool       `json:"reactions"`
	Replies                        bool       `json:"replies"`
	Quotes                         bool       `json:"quotes"`
	Mutes                          bool       `json:"mutes"`
	Uploads                        bool       `json:"uploads"`
	URLEnrichment                  bool       `json:"url_enrichment"`
	CustomEvents                   bool       `json:"custom_events"`
	PushNotifications              bool       `json:"push_notifications"`
	Reminders                      bool       `json:"reminders"`
	MarkMessagesPending            bool       `json:"mark_messages_pending"`
	Polls                          bool       `json:"polls"`
	UserMessageReminders           bool       `json:"user_message_reminders"`
	SharedLocations                bool       `json:"shared_locations"`
	CountMessages                  bool       `json:"count_messages"`
	MessageRetention               string     `json:"message_retention"`
	MaxMessageLength               int        `json:"max_message_length"`
	Automod                        string     `json:"automod"`
	AutomodBehavior                string     `json:"automod_behavior"`
	BlocklistBehavior              string     `json:"blocklist_behavior"`
	SkipLastMsgUpdateForSystemMsgs bool       `json:"skip_last_msg_update_for_system_msgs"`
	Commands                       []*Command `json:"commands"`
}

type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Args        string `json:"args"`
	Set         string `json:"set"`
}

type Message struct {
	ID                   string                    `json:"id"`
	Text                 string                    `json:"text"`
	HTML                 string                    `json:"html"`
	Type                 string                    `json:"type"`
	User                 *UserResponse             `json:"user"`
	Member               *ChannelMember            `json:"member,omitempty"`
	Attachments          []*Attachment             `json:"attachments"`
	LatestReactions      []*Reaction               `json:"latest_reactions"`
	OwnReactions         []*Reaction               `json:"own_reactions"`
	ReactionCounts       map[string]int            `json:"reaction_counts"`
	ReactionScores       map[string]int            `json:"reaction_scores"`
	ReactionGroups       map[string]*ReactionGroup `json:"reaction_groups"`
	ReplyCount           int                       `json:"reply_count"`
	DeletedReplyCount    int                       `json:"deleted_reply_count"`
	CID                  string                    `json:"cid"`
	CreatedAt            *time.Time                `json:"created_at"`
	UpdatedAt            *time.Time                `json:"updated_at"`
	Shadowed             bool                      `json:"shadowed"`
	MentionedUsers       []*UserResponse           `json:"mentioned_users"`
	Silent               bool                      `json:"silent"`
	Pinned               bool                      `json:"pinned"`
	PinnedAt             **time.Time               `json:"pinned_at"`
	PinnedBy             *UserResponse             `json:"pinned_by"`
	PinExpires           **time.Time               `json:"pin_expires"`
	RestrictedVisibility []string                  `json:"restricted_visibility"`
	QuotedMessageID      string                    `json:"quoted_message_id,omitempty"`
	QuotedMessage        *Message                  `json:"quoted_message,omitempty"`
}

type Attachment struct {
	Type          string         `json:"type"`
	Fallback      string         `json:"fallback"`
	ImageURL      string         `json:"image_url"`
	ThumbURL      string         `json:"thumb_url,omitempty"`
	MimeType      string         `json:"mime_type"`
	OriginalImage *OriginalImage `json:"originalImage"`
	AuthorName    string         `json:"author_name,omitempty"`
	Title         string         `json:"title,omitempty"`
	TitleLink     string         `json:"title_link,omitempty"`
	Text          string         `json:"text,omitempty"`
	OGScrapeURL   string         `json:"og_scrape_url,omitempty"`
}

type OriginalImage struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Reaction struct {
	MessageID string        `json:"message_id"`
	UserID    string        `json:"user_id"`
	User      *UserResponse `json:"user"`
	Type      string        `json:"type"`
	Score     int           `json:"score"`
	CreatedAt *time.Time    `json:"created_at"`
	UpdatedAt *time.Time    `json:"updated_at"`
}

type ReactionGroup struct {
	Count           int        `json:"count"`
	SumScores       int        `json:"sum_scores"`
	FirstReactionAt *time.Time `json:"first_reaction_at"`
	LastReactionAt  *time.Time `json:"last_reaction_at"`
}

type Read struct {
	User              *UserResponse `json:"user"`
	LastRead          *time.Time    `json:"last_read"`
	UnreadMessages    int           `json:"unread_messages"`
	LastReadMessageID string        `json:"last_read_message_id"`
}

type ChannelMember struct {
	UserID             string        `json:"user_id,omitempty"`
	User               *UserResponse `json:"user"`
	Status             string        `json:"status"`
	CreatedAt          *time.Time    `json:"created_at"`
	UpdatedAt          *time.Time    `json:"updated_at"`
	Banned             bool          `json:"banned"`
	ShadowBanned       bool          `json:"shadow_banned"`
	Role               string        `json:"role,omitempty"`
	ChannelRole        string        `json:"channel_role"`
	NotificationsMuted bool          `json:"notifications_muted"`
}
