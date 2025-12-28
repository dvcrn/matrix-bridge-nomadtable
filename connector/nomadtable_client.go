package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/dvcrn/matrix-bridge-nomadtable/pkg/nomadtable"
	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/bridgev2/status"
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

	cacheMu     sync.Mutex
	userCache   map[string]*nomadtable.UserResponse
	avatarCache map[string]*bridgev2.Avatar
}

func (nc *NomadtableClient) getAvatar(url string) *bridgev2.Avatar {
	if url == "" {
		return nil
	}

	nc.cacheMu.Lock()
	if nc.avatarCache == nil {
		nc.avatarCache = make(map[string]*bridgev2.Avatar)
	}
	if cached, ok := nc.avatarCache[url]; ok {
		nc.cacheMu.Unlock()
		return cached
	}
	nc.cacheMu.Unlock()

	log := nc.log.With().Str("avatar_url", url).Logger()
	avatar := &bridgev2.Avatar{
		ID: networkid.AvatarID(url),
		Get: func(ctx context.Context) ([]byte, error) {
			log.Debug().Msg("Fetching avatar")
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, fmt.Errorf("create avatar request: %w", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("fetch avatar: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				return nil, fmt.Errorf("fetch avatar: unexpected status %d", resp.StatusCode)
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("read avatar body: %w", err)
			}
			return data, nil
		},
	}

	nc.cacheMu.Lock()
	nc.avatarCache[url] = avatar
	nc.cacheMu.Unlock()

	return avatar
}

func (nc *NomadtableClient) cacheUser(user *nomadtable.UserResponse) {
	if user == nil || user.ID == "" {
		return
	}

	nc.cacheMu.Lock()
	if nc.userCache == nil {
		nc.userCache = make(map[string]*nomadtable.UserResponse)
	}
	existing, ok := nc.userCache[user.ID]
	if !ok {
		clone := *user
		nc.userCache[user.ID] = &clone
		nc.cacheMu.Unlock()
		return
	}
	if user.Name != "" {
		existing.Name = user.Name
	}
	if user.ProfileImage != "" {
		existing.ProfileImage = user.ProfileImage
	}
	if user.Gender != "" {
		existing.Gender = user.Gender
	}
	if user.UpdatedAt != nil {
		existing.UpdatedAt = user.UpdatedAt
	}
	nc.cacheMu.Unlock()
}

func (nc *NomadtableClient) getCachedUser(userID string) *nomadtable.UserResponse {
	if userID == "" {
		return nil
	}

	nc.cacheMu.Lock()
	defer nc.cacheMu.Unlock()
	if nc.userCache == nil {
		return nil
	}
	cached, ok := nc.userCache[userID]
	if !ok || cached == nil {
		return nil
	}
	clone := *cached
	return &clone
}

// Connect starts the websocket connection.
func (nc *NomadtableClient) Connect(ctx context.Context) {
	nc.log.Info().Msg("NomadtableClient Connect called")

	wsCtx, cancel := context.WithCancel(context.Background())
	nc.wsCancel = cancel

	go func() {
		defer cancel()
		nc.runWebsocketLoop(ctx, wsCtx)
	}()
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

func (nc *NomadtableClient) runWebsocketLoop(ctx context.Context, wsCtx context.Context) {
	const reconnectDelay = 10 * time.Second
	firstConnect := true

	for {
		if wsCtx.Err() != nil {
			return
		}

		nc.sendBridgeState(status.BridgeState{StateEvent: status.StateConnecting})

		session, messages, connectionID, err := nc.connectWebsocket(wsCtx)
		if err != nil {
			if wsCtx.Err() != nil {
				return
			}
			nc.log.Err(err).Msg("Failed to connect websocket")
			nc.sendTransientDisconnect("nomadtable_ws_connect_failed", err)
			if !sleepWithContext(wsCtx, reconnectDelay) {
				return
			}
			continue
		}

		nc.session = session
		nc.sendBridgeState(status.BridgeState{StateEvent: status.StateConnected})

		if firstConnect {
			nc.log.Info().Str("connection_id", connectionID).Msg("Websocket connection_id ready")
			firstConnect = false
		} else {
			nc.log.Info().Str("connection_id", connectionID).Msg("Websocket reconnected")
			nc.log.Info().Msg("Triggering ChatResync after websocket reconnect")
		}

		go nc.loadRooms(ctx, connectionID)

		nc.handleWebsocketMessages(ctx, wsCtx, session, messages)
		_ = session.Close()

		if wsCtx.Err() != nil {
			return
		}

		nc.log.Info().
			Int("backoff_seconds", int(reconnectDelay.Seconds())).
			Msg("Websocket disconnected, reconnecting...")

		if !sleepWithContext(wsCtx, reconnectDelay) {
			return
		}
	}
}

func (nc *NomadtableClient) connectWebsocket(ctx context.Context) (*nomadtable.WebsocketSession, <-chan nomadtable.WebsocketMessage, string, error) {
	messages := make(chan nomadtable.WebsocketMessage, 256)
	session, err := nc.client.ConnectWebsocket(ctx, nc.meta.UserID, messages, &nomadtable.WebsocketOptions{
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
		return nil, nil, "", err
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 15*time.Second)
	defer waitCancel()

	nc.log.Info().Str("user_id", nc.meta.UserID).Msg("Waiting for websocket connection_id")
	connectionID, err := waitForConnectionID(waitCtx, session, messages)
	if err != nil {
		_ = session.Close()
		return nil, nil, "", fmt.Errorf("websocket connected but did not yield connection_id: %w", err)
	}
	if connectionID == "" {
		_ = session.Close()
		return nil, nil, "", fmt.Errorf("websocket yielded empty connection_id")
	}

	return session, messages, connectionID, nil
}

func (nc *NomadtableClient) handleWebsocketMessages(ctx context.Context, wsCtx context.Context, session *nomadtable.WebsocketSession, messages <-chan nomadtable.WebsocketMessage) {
	for {
		select {
		case <-wsCtx.Done():
			_ = session.Close()
			return
		case err := <-session.Err():
			if wsCtx.Err() != nil {
				return
			}
			if err != nil {
				nc.log.Err(err).Msg("Websocket error")
			} else {
				nc.log.Warn().Msg("Websocket error channel closed")
			}
			nc.sendTransientDisconnect("nomadtable_ws_error", err)
			return
		case <-session.Done():
			if wsCtx.Err() != nil {
				return
			}
			nc.log.Info().Msg("Websocket session done")
			nc.sendTransientDisconnect("nomadtable_ws_closed", nil)
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
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (nc *NomadtableClient) sendBridgeState(state status.BridgeState) {
	if nc.login == nil || nc.login.BridgeState == nil {
		return
	}
	nc.login.BridgeState.Send(state)
}

func (nc *NomadtableClient) sendTransientDisconnect(code status.BridgeStateErrorCode, err error) {
	state := status.BridgeState{
		StateEvent: status.StateTransientDisconnect,
		Error:      code,
	}
	if err != nil {
		state.Info = map[string]any{
			"error": err.Error(),
		}
	}
	nc.sendBridgeState(state)
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
	case "notification.added_to_channel":
		nc.log.Info().Str("cid", base.CID).Msg("Added to channel, triggering portal resync")
		if nc.session == nil {
			nc.log.Error().Msg("Resync skipped: no websocket session")
			return nil
		}
		connectionID := nc.session.ConnectionID()
		if connectionID == "" {
			nc.log.Error().Msg("Resync skipped: missing connection_id")
			return nil
		}
		go nc.loadRooms(ctx, connectionID)
		return nil
	default:
		return nil
	}
}

func (nc *NomadtableClient) handleRemoteMessage(ctx context.Context, ev *nomadtable.NotificationMessageNew) error {
	if ev == nil {
		return nil
	}

	nc.cacheUser(ev.Message.User)

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

	createPortal := true
	portal, err := nc.bridge.GetPortalByKey(ctx, portalKey)
	if err != nil {
		nc.log.Err(err).Str("portal_key", string(portalKey.ID)).Msg("Failed to get/provision portal for incoming message")
	} else {
		createPortal = portal.MXID == ""
	}

	// Always resync on incoming messages to ensure we pick up any missed history.
	nc.bridge.QueueRemoteEvent(nc.login, &simplevent.ChatResync{
		EventMeta: simplevent.EventMeta{
			Type:         bridgev2.RemoteEventChatResync,
			PortalKey:    portalKey,
			CreatePortal: createPortal,
			Timestamp:    ts,
		},
		ChatInfo:        nil,
		LatestMessageTS: ts,
	})

	nc.log.Info().
		Str("portal_key", string(portalKey.ID)).
		Str("message_id", msgID).
		Bool("create_portal", createPortal).
		Msg("Queued ChatResync for incoming message")

	return nil
}

func (nc *NomadtableClient) loadRooms(ctx context.Context, connectionID string) {
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
		if state.Channel != nil {
			nc.cacheUser(state.Channel.CreatedBy)
		}
		if state.Membership != nil {
			nc.cacheUser(state.Membership.User)
		}
		for _, member := range state.Members {
			if member != nil {
				nc.cacheUser(member.User)
			}
		}
		for _, read := range state.Read {
			if read != nil {
				nc.cacheUser(read.User)
			}
		}
		for _, msg := range state.Messages {
			if msg != nil {
				nc.cacheUser(msg.User)
				nc.cacheUser(msg.PinnedBy)
			}
		}
		for _, msg := range state.PinnedMessages {
			if msg != nil {
				nc.cacheUser(msg.User)
				nc.cacheUser(msg.PinnedBy)
			}
		}

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
