package connector

import (
	"context"
	"fmt"

	"github.com/dvcrn/matrix-bridge-nomadtable/pkg/nomadtable"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
)

// HandleMatrixMessage handles incoming messages from Matrix for this user.
func (nc *NomadtableClient) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (*bridgev2.MatrixMessageResponse, error) {
	log := nc.log.With().
		Str("portal_id", string(msg.Portal.ID)).
		Str("sender_mxid", string(msg.Event.Sender)).
		Str("event_id", string(msg.Event.ID)).
		Logger()
	ctx = log.WithContext(ctx)

	log.Info().Msg("HandleMatrixMessage called")

	_, err := nc.bridge.GetExistingUserByMXID(ctx, msg.Event.Sender)
	if err != nil {
		log.Err(err).Str("user_mxid", string(msg.Event.Sender)).Msg("Failed to get user object, ignoring message")
		return nil, nil
	}

	if msg.Content == nil {
		log.Debug().Msg("Ignoring Matrix message with nil content")
		return nil, nil
	}
	if msg.Content.MsgType != event.MsgText && msg.Content.MsgType != event.MsgNotice && msg.Content.MsgType != event.MsgEmote {
		log.Debug().Str("msgtype", string(msg.Content.MsgType)).Msg("Ignoring unsupported Matrix msgtype")
		return nil, nil
	}

	body := msg.Content.Body
	if body == "" {
		log.Debug().Msg("Ignoring empty Matrix message")
		return nil, nil
	}

	if nc.session == nil {
		return nil, fmt.Errorf("no websocket session")
	}
	connectionID := nc.session.ConnectionID()
	if connectionID == "" {
		return nil, fmt.Errorf("missing connection_id")
	}

	channelType, channelID, err := parsePortalKeyToChannel(msg.Portal.PortalKey.ID)
	if err != nil {
		return nil, err
	}

	preview := body
	if len(preview) > 200 {
		preview = preview[:200] + "…"
	}
	log.Info().
		Str("channel_type", channelType).
		Str("channel_id", channelID).
		Str("connection_id", connectionID).
		Str("user_id", nc.meta.UserID).
		Int("len", len(body)).
		Str("text", preview).
		Msg("Sending Matrix message to Nomadtable")

	resp, err := nc.client.SendMessage(ctx, channelType, channelID, nc.meta.UserID, connectionID, &nomadtable.SendMessageRequest{
		Message: &nomadtable.MessageInput{
			Text: body,
		},
	})
	if err != nil {
		log.Err(err).Msg("SendMessage failed")
		return nil, err
	}

	remoteID := ""
	if resp != nil && resp.Message != nil {
		remoteID = resp.Message.ID
	}
	log.Info().
		Str("remote_message_id", remoteID).
		Bool("has_message", resp != nil && resp.Message != nil).
		Msg("SendMessage completed")

	if remoteID == "" {
		log.Warn().Msg("SendMessage succeeded but response had no message ID")
		return &bridgev2.MatrixMessageResponse{DB: &database.Message{ID: networkid.MessageID("unknown")}}, nil
	}

	return &bridgev2.MatrixMessageResponse{DB: &database.Message{ID: networkid.MessageID(remoteID)}}, nil
}

// GetUserInfo resolves user info from cached data or channel state.
func (nc *NomadtableClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	if ghost == nil {
		return nil, fmt.Errorf("ghost is nil")
	}

	log := nc.log.With().Str("ghost_id", string(ghost.ID)).Logger()
	ctx = log.WithContext(ctx)

	userID := string(ghost.ID)
	if cached := nc.getCachedUser(userID); cached != nil {
		log.Info().Msg("User cache hit")
		name := cached.Name
		if name == "" {
			name = cached.ID
		}
		info := &bridgev2.UserInfo{Name: ptr.Ptr(name)}
		if cached.ProfileImage != "" {
			info.Avatar = nc.getAvatar(cached.ProfileImage)
		}
		return info, nil
	}

	log.Info().Msg("User cache miss, querying channels")

	if nc.session == nil {
		return nil, fmt.Errorf("no websocket session")
	}
	connectionID := nc.session.ConnectionID()
	if connectionID == "" {
		return nil, fmt.Errorf("missing connection_id")
	}

	filter := map[string]any{
		"members": map[string]any{
			"$in": []string{nc.meta.UserID},
		},
		"$or": []any{
			map[string]any{
				"plan_id": map[string]any{"$exists": true},
			},
			map[string]any{
				"$and": []any{
					map[string]any{"member_count": 2},
					map[string]any{"last_message_at": map[string]any{"$exists": true}},
				},
			},
			map[string]any{
				"member_count": map[string]any{"$gt": 2},
			},
		},
	}

	matchUser := func(user *nomadtable.UserResponse) *nomadtable.UserResponse {
		if user == nil {
			return nil
		}
		nc.cacheUser(user)
		if user.ID == userID {
			return user
		}
		return nil
	}

	const pageLimit = 100
	offset := 0
	for {
		resp, err := nc.client.QueryChannels(ctx, nc.meta.UserID, connectionID, &nomadtable.QueryChannelsRequest{
			FilterConditions: filter,
			Sort: []*nomadtable.SortOption{
				{Field: "pinned_at", Direction: -1},
				{Field: "updated_at", Direction: -1},
			},
			State:        true,
			Watch:        false,
			Presence:     false,
			Limit:        pageLimit,
			Offset:       offset,
			MessageLimit: 50,
		})
		if err != nil {
			return nil, fmt.Errorf("QueryChannels failed: %w", err)
		}

		for _, state := range resp.Channels {
			if state.Channel != nil {
				if user := matchUser(state.Channel.CreatedBy); user != nil {
					log.Info().Msg("Resolved user info from channel creator")
					break
				}
			}
			if state.Membership != nil {
				if user := matchUser(state.Membership.User); user != nil {
					log.Info().Msg("Resolved user info from channel membership")
					break
				}
			}
			for _, member := range state.Members {
				if member != nil {
					if user := matchUser(member.User); user != nil {
						log.Info().Msg("Resolved user info from channel members")
						break
					}
				}
			}
			for _, read := range state.Read {
				if read != nil {
					if user := matchUser(read.User); user != nil {
						log.Info().Msg("Resolved user info from channel read state")
						break
					}
				}
			}
			for _, msg := range state.Messages {
				if msg != nil {
					if user := matchUser(msg.User); user != nil {
						log.Info().Msg("Resolved user info from channel messages")
						break
					}
					if user := matchUser(msg.PinnedBy); user != nil {
						log.Info().Msg("Resolved user info from pinned message")
						break
					}
				}
			}
			for _, msg := range state.PinnedMessages {
				if msg != nil {
					if user := matchUser(msg.User); user != nil {
						log.Info().Msg("Resolved user info from pinned messages")
						break
					}
					if user := matchUser(msg.PinnedBy); user != nil {
						log.Info().Msg("Resolved user info from pinned message")
						break
					}
				}
			}
		}

		if cached := nc.getCachedUser(userID); cached != nil {
			log.Info().Msg("User cache filled from channel scan")
			name := cached.Name
			if name == "" {
				name = cached.ID
			}
			info := &bridgev2.UserInfo{Name: ptr.Ptr(name)}
			if cached.ProfileImage != "" {
				info.Avatar = nc.getAvatar(cached.ProfileImage)
			}
			return info, nil
		}

		if len(resp.Channels) < pageLimit {
			break
		}
		offset += pageLimit
	}

	return nil, fmt.Errorf("user info not found in channel state")
}

// GetChatInfo returns the full desired Matrix room state for the portal.
func (nc *NomadtableClient) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	log := nc.log.With().
		Str("portal_key", string(portal.PortalKey.ID)).
		Str("portal_mxid", string(portal.MXID)).
		Logger()
	ctx = log.WithContext(ctx)

	log.Info().Msg("GetChatInfo called")

	if nc.session == nil {
		log.Error().Msg("GetChatInfo called without websocket session")
		return nil, fmt.Errorf("no websocket session")
	}
	connectionID := nc.session.ConnectionID()
	if connectionID == "" {
		log.Error().Msg("GetChatInfo called without connection_id")
		return nil, fmt.Errorf("missing connection_id")
	}

	channelType, channelID, err := parsePortalKeyToChannel(portal.PortalKey.ID)
	if err != nil {
		log.Err(err).Msg("Failed to parse portal key into channel")
		return nil, err
	}

	log.Info().
		Str("channel_type", channelType).
		Str("channel_id", channelID).
		Str("connection_id", connectionID).
		Msg("Fetching channel state via QueryChannel")

	resp, err := nc.client.QueryChannel(ctx, channelType, channelID, nc.meta.UserID, connectionID, &nomadtable.QueryChannelRequest{
		State:    true,
		Watch:    false,
		Presence: false,
	})
	if err != nil {
		log.Err(err).Msg("QueryChannel failed")
		return nil, fmt.Errorf("QueryChannel failed: %w", err)
	}
	if resp.Channel == nil {
		log.Error().Msg("QueryChannel returned nil channel")
		return nil, fmt.Errorf("QueryChannel returned nil channel")
	}

	nc.cacheUser(resp.Channel.CreatedBy)
	if resp.Membership != nil {
		nc.cacheUser(resp.Membership.User)
	}
	for _, member := range resp.Members {
		if member != nil {
			nc.cacheUser(member.User)
		}
	}
	for _, read := range resp.Read {
		if read != nil {
			nc.cacheUser(read.User)
		}
	}
	for _, msg := range resp.Messages {
		if msg != nil {
			nc.cacheUser(msg.User)
			nc.cacheUser(msg.PinnedBy)
		}
	}
	for _, msg := range resp.PinnedMessages {
		if msg != nil {
			nc.cacheUser(msg.User)
			nc.cacheUser(msg.PinnedBy)
		}
	}

	log.Info().
		Str("cid", resp.Channel.CID).
		Str("name", resp.Channel.Name).
		Int("member_count", resp.Channel.MemberCount).
		Int("members", len(resp.Members)).
		Msg("QueryChannel returned channel info")

	ch := resp.Channel
	name := ch.Name
	if name == "" {
		name = ch.CID
		if name == "" {
			name = fmt.Sprintf("%s:%s", ch.Type, ch.ID)
		}
	}

	topic := fmt.Sprintf("Nomadtable channel %s", ch.CID)
	if ch.PlanID != "" {
		topic = fmt.Sprintf("plan_id=%s cid=%s", ch.PlanID, ch.CID)
	}

	chatInfo := &bridgev2.ChatInfo{
		Name:   ptr.Ptr(name),
		Topic:  ptr.Ptr(topic),
		Avatar: nc.getAvatar(ch.Image),
	}

	if ch.MemberCount == 2 {
		rt := database.RoomTypeDM
		chatInfo.Type = &rt
	} else if ch.MemberCount > 2 {
		rt := database.RoomTypeGroupDM
		chatInfo.Type = &rt
	}

	// Populate member list from the channel state when available.
	if len(resp.Members) > 0 {
		members := make([]bridgev2.ChatMember, 0, len(resp.Members))
		for _, m := range resp.Members {
			if m == nil {
				continue
			}
			if m.User == nil || m.User.ID == "" {
				continue
			}

			remoteID := networkid.UserID(m.User.ID)
			display := m.User.Name
			if display == "" {
				display = m.User.ID
			}

			members = append(members, bridgev2.ChatMember{
				EventSender: bridgev2.EventSender{Sender: remoteID},
				Membership:  event.MembershipJoin,
				UserInfo: &bridgev2.UserInfo{
					Name:   ptr.Ptr(display),
					Avatar: nc.getAvatar(m.User.ProfileImage),
				},
			})
		}

		if len(members) > 0 {
			chatInfo.Members = &bridgev2.ChatMemberList{
				IsFull:           true,
				Members:          members,
				TotalMemberCount: len(members),
			}
		}
	}

	log.Info().
		Str("resolved_name", name).
		Str("resolved_topic", topic).
		Bool("has_members", chatInfo.Members != nil).
		Msg("GetChatInfo returning")

	return chatInfo, nil
}

// GetCapabilities returns the supported features for chats handled by this client.
func (nc *NomadtableClient) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *event.RoomFeatures {
	return &event.RoomFeatures{
		MaxTextLength: 65536,
		Formatting: event.FormattingFeatureMap{
			event.FmtBold:          event.CapLevelFullySupported,
			event.FmtItalic:        event.CapLevelFullySupported,
			event.FmtUnderline:     event.CapLevelFullySupported,
			event.FmtStrikethrough: event.CapLevelFullySupported,
			event.FmtInlineCode:    event.CapLevelFullySupported,
			event.FmtCodeBlock:     event.CapLevelFullySupported,
		},
		Edit:   event.CapLevelFullySupported,
		Reply:  event.CapLevelFullySupported,
		Thread: event.CapLevelFullySupported,
	}
}
