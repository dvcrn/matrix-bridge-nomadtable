package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dvcrn/matrix-bridge-nomadtable/pkg/nomadtable"
	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
)

// Ensure NomadtableClient implements NetworkAPI.
var _ bridgev2.NetworkAPI = (*NomadtableClient)(nil)

// NomadtableClient implements the bridgev2.NetworkAPI for interacting
// with Nomadtable on behalf of a specific user login.
type NomadtableClient struct {
	log       zerolog.Logger
	bridge    *bridgev2.Bridge
	login     *bridgev2.UserLogin
	connector *NomadtableConnector

	meta   *LoginMetadata
	client *nomadtable.Client

	wsCancel context.CancelFunc
	session  *nomadtable.WebsocketSession
}

// Connect starts the websocket connection.
func (nc *NomadtableClient) Connect(ctx context.Context) {
	nc.log.Info().Msg("NomadtableClient Connect called")

	wsCtx, cancel := context.WithCancel(context.Background())
	nc.wsCancel = cancel

	messages := make(chan nomadtable.WebsocketMessage, 256)
	session, err := nc.client.ConnectWebsocket(wsCtx, nc.meta.UserID, messages, &nomadtable.WebsocketOptions{
		XStreamClient:    "stream-chat-go-client",
		HandshakeTimeout: 30 * time.Second,
		PongWait:         60 * time.Second,
		PingInterval:     25 * time.Second,
		WriteWait:        10 * time.Second,
		Logger: func(format string, args ...any) {
			nc.log.Debug().Msgf(format, args...)
		},
	})
	if err != nil {
		nc.log.Err(err).Msg("Failed to connect websocket")
		cancel()
		return
	}

	nc.session = session

	waitCtx, waitCancel := context.WithTimeout(wsCtx, 15*time.Second)
	defer waitCancel()

	nc.log.Info().Str("user_id", nc.meta.UserID).Msg("Waiting for websocket connection_id")
	connectionID, err := waitForConnectionID(waitCtx, session, messages)
	if err != nil {
		nc.log.Err(err).Msg("Websocket connected but did not yield connection_id")
		_ = session.Close()
		cancel()
		return
	}
	if connectionID == "" {
		nc.log.Error().Msg("Websocket yielded empty connection_id")
		_ = session.Close()
		cancel()
		return
	}
	nc.log.Info().Str("connection_id", connectionID).Msg("Websocket connection_id ready")

	go func() {
		defer cancel()
		for {
			select {
			case <-wsCtx.Done():
				_ = session.Close()
				return
			case err := <-session.Err():
				nc.log.Err(err).Msg("Websocket error")
				return
			case <-session.Done():
				nc.log.Info().Msg("Websocket session done")
				return
			case msg := <-messages:
				nc.log.Debug().
					Time("received_at", msg.ReceivedAt).
					Int("type", msg.Type).
					Int("size", len(msg.Data)).
					Msg("Received websocket message")

				if msg.Type != 1 {
					continue
				}
				if err := nc.handleWebsocketEvent(ctx, msg.Data); err != nil {
					nc.log.Err(err).Msg("Failed to handle websocket event")
				}
			}
		}
	}()

	nc.loadRooms(ctx)
}

func waitForConnectionID(ctx context.Context, session *nomadtable.WebsocketSession, messages <-chan nomadtable.WebsocketMessage) (string, error) {
	for {
		if connectionID := session.ConnectionID(); connectionID != "" {
			return connectionID, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-session.Done():
			return "", fmt.Errorf("websocket closed before connection_id")
		case err := <-session.Err():
			return "", err
		case <-messages:
			// session.ConnectionID() is populated by the websocket reader.
		}
	}
}

func (nc *NomadtableClient) handleWebsocketEvent(ctx context.Context, data []byte) error {
	var base struct {
		Type string `json:"type"`
		CID  string `json:"cid"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		nc.log.Warn().Err(err).Int("bytes", len(data)).Msg("Failed to decode websocket JSON")
		return fmt.Errorf("unmarshal base event: %w", err)
	}
	if base.Type == "" {
		nc.log.Debug().Int("bytes", len(data)).Msg("Websocket event missing type")
		return nil
	}

	// For debugging: log all event types at DEBUG.
	nc.log.Debug().
		Str("event_type", base.Type).
		Str("cid", base.CID).
		Int("bytes", len(data)).
		Msg("Decoded websocket event")

	switch base.Type {
	case "notification.message_new", "message.new":
		var ev nomadtable.NotificationMessageNew
		if err := json.Unmarshal(data, &ev); err != nil {
			return fmt.Errorf("unmarshal message_new: %w", err)
		}
		return nc.handleRemoteMessage(ctx, &ev)
	default:
		return nil
	}
}

func (nc *NomadtableClient) handleRemoteMessage(ctx context.Context, ev *nomadtable.NotificationMessageNew) error {
	if ev == nil {
		return nil
	}

	cid := ev.CID
	if cid == "" {
		cid = ev.Channel.CID
	}
	if cid == "" {
		cid = fmt.Sprintf("%s:%s", ev.ChannelType, ev.ChannelID)
	}
	if cid == "" {
		return fmt.Errorf("missing cid in message.new")
	}

	portalKey := networkid.PortalKey{ID: networkid.PortalID(cid)}
	senderID := ""
	if ev.Message.User != nil {
		senderID = ev.Message.User.ID
	}
	if senderID == "" {
		senderID = "unknown"
	}

	isFromMe := nc.IsThisUser(ctx, networkid.UserID(senderID))

	ts := time.Now()
	if ev.Message.CreatedAt != nil {
		ts = *ev.Message.CreatedAt
	} else if ev.CreatedAt != nil {
		ts = *ev.CreatedAt
	}

	msgID := ev.Message.ID
	if msgID == "" {
		msgID = ev.MessageID
	}
	if msgID == "" {
		msgID = fmt.Sprintf("ws-%d", time.Now().UnixNano())
	}

	body := ev.Message.Text
	if body == "" {
		body = ev.Message.HTML
	}
	if len(body) > 200 {
		body = body[:200] + "…"
	}

	nc.log.Info().
		Str("ws_event", ev.Type).
		Str("cid", cid).
		Str("portal_key", string(portalKey.ID)).
		Str("message_id", msgID).
		Str("sender_id", senderID).
		Bool("is_from_me", isFromMe).
		Str("text", body).
		Msg("Upstream message event received")

	remoteMsg := &NomadtableRemoteMessage{
		EventMeta: simplevent.EventMeta{
			Type:         bridgev2.RemoteEventMessage,
			PortalKey:    portalKey,
			CreatePortal: true,
			Timestamp:    ts,
			Sender: bridgev2.EventSender{
				Sender:   networkid.UserID(senderID),
				IsFromMe: isFromMe,
			},
		},
		Msg:       &ev.Message,
		MessageID: networkid.MessageID(msgID),
	}

	nc.bridge.QueueRemoteEvent(nc.login, remoteMsg)
	nc.log.Info().
		Str("portal_key", string(portalKey.ID)).
		Str("message_id", msgID).
		Msg("Queued incoming Nomadtable message")

	return nil
}

func (nc *NomadtableClient) loadRooms(ctx context.Context) {
	if nc.session == nil {
		nc.log.Error().Msg("loadRooms called without websocket session")
		return
	}
	connectionID := nc.session.ConnectionID()
	if connectionID == "" {
		nc.log.Error().Msg("loadRooms called without connection_id")
		return
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

	req := &nomadtable.QueryChannelsRequest{
		FilterConditions: filter,
		Sort: []*nomadtable.SortOption{
			{Field: "pinned_at", Direction: -1},
			{Field: "updated_at", Direction: -1},
		},
		State:        true,
		Watch:        true,
		Presence:     true,
		Limit:        30,
		Offset:       0,
		MessageLimit: 100,
	}

	nc.log.Info().
		Str("user_id", nc.meta.UserID).
		Str("connection_id", connectionID).
		Msg("Loading Nomadtable channels")

	resp, err := nc.client.QueryChannels(ctx, nc.meta.UserID, connectionID, req)
	if err != nil {
		nc.log.Err(err).Msg("QueryChannels failed")
		return
	}

	nc.log.Info().Int("channels", len(resp.Channels)).Msg("Received Nomadtable channels")

	for _, state := range resp.Channels {
		ch := state.Channel
		if ch == nil {
			continue
		}

		portalID := fmt.Sprintf("%s:%s", ch.Type, ch.ID)
		portalKey := networkid.PortalKey{ID: networkid.PortalID(portalID)}

		nc.log.Debug().
			Str("cid", ch.CID).
			Str("channel_type", ch.Type).
			Str("channel_id", ch.ID).
			Str("channel_name", ch.Name).
			Str("portal_key", string(portalKey.ID)).
			Msg("Ensuring portal exists")

		portal, err := nc.bridge.GetPortalByKey(ctx, portalKey)
		if err != nil {
			nc.log.Err(err).Str("portal_key", string(portalKey.ID)).Msg("Failed to get/provision portal")
			continue
		}

		createPortal := portal.MXID == ""
		nc.log.Info().
			Str("portal_key", string(portalKey.ID)).
			Str("portal_mxid", string(portal.MXID)).
			Bool("create_portal", createPortal).
			Msg("Portal status")

		name := ch.Name
		if name == "" {
			name = ch.CID
			if name == "" {
				name = portalID
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

		latestTS := time.Now()
		if ch.UpdatedAt != nil {
			latestTS = *ch.UpdatedAt
		} else if ch.LastMessageAt != nil {
			latestTS = *ch.LastMessageAt
		}

		nc.bridge.QueueRemoteEvent(nc.login, &simplevent.ChatResync{
			EventMeta: simplevent.EventMeta{
				Type:         bridgev2.RemoteEventChatResync,
				PortalKey:    portalKey,
				CreatePortal: createPortal,
				Timestamp:    latestTS,
			},
			ChatInfo:        chatInfo,
			LatestMessageTS: latestTS,
		})

		nc.log.Info().
			Str("portal_key", string(portalKey.ID)).
			Bool("create_portal", createPortal).
			Str("name", name).
			Int("member_count", ch.MemberCount).
			Msg("Queued ChatResync for channel")
	}
}

func (nc *NomadtableClient) Disconnect() {
	nc.log.Info().Msg("NomadtableClient Disconnect called")
	if nc.wsCancel != nil {
		nc.wsCancel()
		nc.wsCancel = nil
	}
}

func (nc *NomadtableClient) LogoutRemote(ctx context.Context) {
	nc.log.Info().Msg("NomadtableClient LogoutRemote called")
}

// IsThisUser checks if the given remote network user ID belongs to this client instance.
func (nc *NomadtableClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	return string(userID) == nc.login.RemoteName
}

// IsLoggedIn always returns true for this simple connector.
func (nc *NomadtableClient) IsLoggedIn() bool {
	return true
}
