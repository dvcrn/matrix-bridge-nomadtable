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

	// Placeholder: demonstrate Remote -> Matrix flow by echoing a message.
	nc.QueueRemoteMessage(ctx, msg.Portal.ID, "Hi there too")

	return &bridgev2.MatrixMessageResponse{}, nil
}

// GetUserInfo is not implemented for this simple connector.
func (nc *NomadtableClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	return nil, fmt.Errorf("user info not available")
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
		Name:  ptr.Ptr(name),
		Topic: ptr.Ptr(topic),
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
				UserInfo:    &bridgev2.UserInfo{Name: ptr.Ptr(display)},
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
