package nomadtable

import "time"

// WebSocketEvent represents a base websocket event
type WebSocketEvent struct {
	Type      string     `json:"type"`
	CreatedAt *time.Time `json:"created_at"`
	CID       string     `json:"cid,omitempty"`
}

// Member represents a channel member
type Member struct {
	UserID             string     `json:"user_id"`
	User               User       `json:"user"`
	Status             string     `json:"status"`
	CreatedAt          *time.Time `json:"created_at"`
	UpdatedAt          *time.Time `json:"updated_at"`
	Banned             bool       `json:"banned"`
	ShadowBanned       bool       `json:"shadow_banned"`
	Role               string     `json:"role"`
	ChannelRole        string     `json:"channel_role"`
	NotificationsMuted bool       `json:"notifications_muted"`
}

// MessageMember represents a simplified member info in a message
type MessageMember struct {
	ChannelRole string `json:"channel_role"`
}

// ChannelCustom represents custom channel fields in events.
type ChannelCustom struct {
	ActivityEmoji string `json:"activityEmoji"`
	Name          string `json:"name"`
	PlanID        string `json:"plan_id"`
}

// ChannelKickedEvent represents a user being kicked from a channel.
type ChannelKickedEvent struct {
	Type               string         `json:"type"`
	CreatedAt          *time.Time     `json:"created_at"`
	CID                string         `json:"cid"`
	ChannelMemberCount int            `json:"channel_member_count"`
	ChannelCustom      *ChannelCustom `json:"channel_custom"`
	ChannelType        string         `json:"channel_type"`
	ChannelID          string         `json:"channel_id"`
}

// NotificationMessageNew represents a new message notification event
type NotificationMessageNew struct {
	Type               string     `json:"type"`
	CreatedAt          *time.Time `json:"created_at"`
	CID                string     `json:"cid"`
	ChannelMemberCount int        `json:"channel_member_count"`
	ChannelType        string     `json:"channel_type"`
	ChannelID          string     `json:"channel_id"`
	Channel            Channel    `json:"channel"`
	MessageID          string     `json:"message_id"`
	Message            Message    `json:"message"`
	WatcherCount       int        `json:"watcher_count"`
	UnreadCount        int        `json:"unread_count"`
	TotalUnreadCount   int        `json:"total_unread_count"`
	UnreadChannels     int        `json:"unread_channels"`
}

// NotificationMarkRead represents a mark read notification event
type NotificationMarkRead struct {
	Type                 string      `json:"type"`
	CreatedAt            *time.Time  `json:"created_at"`
	CID                  string      `json:"cid"`
	ChannelMemberCount   int         `json:"channel_member_count"`
	ChannelType          string      `json:"channel_type"`
	ChannelID            string      `json:"channel_id"`
	Channel              Channel     `json:"channel"`
	User                 User        `json:"user"`
	LastReadMessageID    string      `json:"last_read_message_id"`
	UnreadCount          int         `json:"unread_count"`
	TotalUnreadCount     int         `json:"total_unread_count"`
	UnreadChannels       int         `json:"unread_channels"`
	UnreadThreads        interface{} `json:"unread_threads"`
	UnreadThreadMessages interface{} `json:"unread_thread_messages"`
}

// NotificationAddedToChannel represents a user being added to a channel.
type NotificationAddedToChannel struct {
	Type               string         `json:"type"`
	CreatedAt          *time.Time     `json:"created_at"`
	CID                string         `json:"cid"`
	ChannelMemberCount int            `json:"channel_member_count"`
	ChannelType        string         `json:"channel_type"`
	ChannelID          string         `json:"channel_id"`
	Channel            *Channel       `json:"channel"`
	Member             *ChannelMember `json:"member"`
	UnreadCount        int            `json:"unread_count"`
	TotalUnreadCount   int            `json:"total_unread_count"`
	UnreadChannels     int            `json:"unread_channels"`
}

// MessageReadEvent represents a message read event.
type MessageReadEvent struct {
	Type                 string         `json:"type"`
	CreatedAt            *time.Time     `json:"created_at"`
	CID                  string         `json:"cid"`
	ChannelMemberCount   int            `json:"channel_member_count"`
	ChannelCustom        *ChannelCustom `json:"channel_custom"`
	ChannelType          string         `json:"channel_type"`
	ChannelID            string         `json:"channel_id"`
	ChannelLastMessageAt *time.Time     `json:"channel_last_message_at"`
	User                 *UserResponse  `json:"user"`
	LastReadMessageID    string         `json:"last_read_message_id"`
}

// TypingStartEvent represents a typing start event.
type TypingStartEvent struct {
	Type                 string        `json:"type"`
	CID                  string        `json:"cid"`
	ChannelID            string        `json:"channel_id"`
	ChannelType          string        `json:"channel_type"`
	ChannelLastMessageAt *time.Time    `json:"channel_last_message_at"`
	User                 *UserResponse `json:"user"`
	CreatedAt            *time.Time    `json:"created_at"`
}

// TypingStopEvent represents a typing stop event.
type TypingStopEvent struct {
	Type                 string        `json:"type"`
	CID                  string        `json:"cid"`
	ChannelID            string        `json:"channel_id"`
	ChannelType          string        `json:"channel_type"`
	ChannelLastMessageAt *time.Time    `json:"channel_last_message_at"`
	User                 *UserResponse `json:"user"`
	CreatedAt            *time.Time    `json:"created_at"`
}

// HealthCheck represents a health check event
type HealthCheck struct {
	ConnectionID string     `json:"connection_id"`
	CID          string     `json:"cid"`
	Type         string     `json:"type"`
	CreatedAt    *time.Time `json:"created_at"`
}
