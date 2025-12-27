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

// HealthCheck represents a health check event
type HealthCheck struct {
	ConnectionID string     `json:"connection_id"`
	CID          string     `json:"cid"`
	Type         string     `json:"type"`
	CreatedAt    *time.Time `json:"created_at"`
}
