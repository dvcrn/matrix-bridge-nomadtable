package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dvcrn/matrix-bridge-nomadtable/pkg/nomadtable"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
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

func (nc *NomadtableClient) loadRooms(ctx context.Context) {
	filter := map[string]any{
		"members": map[string]any{
			"$in": []string{nc.meta.UserID},
		},
		"$or": []any{
			map[string]any{
				"plan_id": map[string]any{
					"$exists": true,
				},
			},
			map[string]any{
				"$and": []any{
					map[string]any{"member_count": 2},
					map[string]any{
						"last_message_at": map[string]any{
							"$exists": true,
						},
					},
				},
			},
			map[string]any{
				"member_count": map[string]any{
					"$gt": 2,
				},
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

	fmt.Printf("Loading room with: userid=%s, connectionid=%s\n", nc.meta.UserID, nc.session.ConnectionID())
	fmt.Printf("QueryChannels: %v\n", req)
	resp, err := nc.client.QueryChannels(ctx, nc.meta.UserID, nc.session.ConnectionID(), req)
	if err != nil {
		panic(fmt.Sprintf("QueryChannels failed: %v", err))
	}

	func(v interface{}) {
		j, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}
		buf := bytes.NewBuffer(j)
		fmt.Printf("%v\n", buf.String())
	}(resp)
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
