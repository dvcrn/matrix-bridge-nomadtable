package nomadtable

import "time"

type UpdateUserResponse struct {
	Users                    map[string]*UserResponse `json:"users"`
	MembershipDeletionTaskID string                   `json:"membership_deletion_task_id"`
	Duration                 string                   `json:"duration"`
}

type UserResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Language         string    `json:"language"`
	Role             string    `json:"role"`
	Teams            []any     `json:"teams"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Banned           bool      `json:"banned"`
	Online           bool      `json:"online"`
	LastActive       time.Time `json:"last_active"`
	Devices          []*Device `json:"devices"`
	Invisible        bool      `json:"invisible"`
	Mutes            []any     `json:"mutes"`
	ChannelMutes     []any     `json:"channel_mutes"`
	UnreadCount      int       `json:"unread_count"`
	TotalUnreadCount int       `json:"total_unread_count"`
	UnreadChannels   int       `json:"unread_channels"`
	UnreadThreads    int       `json:"unread_threads"`
	ShadowBanned     bool      `json:"shadow_banned"`
	BlockedUserIDs   []any     `json:"blocked_user_ids"`
	Gender           string    `json:"gender"`
	ProfileImage     string    `json:"profileImage"`
}

type Device struct {
	PushProvider     string    `json:"push_provider"`
	PushProviderName string    `json:"push_provider_name"`
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UserID           string    `json:"user_id"`
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
