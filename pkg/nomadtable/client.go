package nomadtable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	BaseURL    string
	APIKey     string
	AuthToken  string
	HTTPClient *http.Client
}

func NewClient(apiKey, authToken string) *Client {
	return &Client{
		BaseURL:    "https://chat.stream-io-api.com",
		APIKey:     apiKey,
		AuthToken:  authToken,
		HTTPClient: http.DefaultClient,
	}
}

type User struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProfileImage string `json:"profileImage"`
	Gender       string `json:"gender"`
}

func (c *Client) UpdateUser(ctx context.Context, user *User) (*UpdateUserResponse, error) {
	payload := map[string]map[string]*User{
		"users": {
			user.ID: user,
		},
	}

	q := url.Values{}

	var result UpdateUserResponse
	if err := c.do(ctx, http.MethodPost, "/users", q, payload, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetApp(ctx context.Context, userID, connectionID string) (*GetAppResponse, error) {
	q := url.Values{}
	q.Set("user_id", userID)
	q.Set("connection_id", connectionID)

	var result GetAppResponse
	if err := c.do(ctx, http.MethodGet, "/app", q, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type QueryChannelsRequest struct {
	FilterConditions map[string]any `json:"filter_conditions"`
	Sort             []*SortOption  `json:"sort"`
	State            bool           `json:"state"`
	Watch            bool           `json:"watch"`
	Presence         bool           `json:"presence"`
	Limit            int            `json:"limit"`
	Offset           int            `json:"offset"`
	MessageLimit     int            `json:"message_limit"`
}

type SortOption struct {
	Field     string `json:"field"`
	Direction int    `json:"direction"`
}

func (c *Client) QueryChannels(ctx context.Context, userID, connectionID string, request *QueryChannelsRequest) (*QueryChannelsResponse, error) {
	q := url.Values{}
	q.Set("user_id", userID)
	q.Set("connection_id", connectionID)

	var result QueryChannelsResponse
	if err := c.do(ctx, http.MethodPost, "/channels", q, request, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) do(ctx context.Context, method, path string, params url.Values, body any, result any) error {
	u := c.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	for k, v := range params {
		for _, val := range v {
			q.Add(k, val)
		}
	}
	q.Set("api_key", c.APIKey)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", c.AuthToken)
	req.Header.Set("stream-auth-type", "jwt")
	req.Header.Set("x-stream-client", "stream-chat-go-client")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

type WebsocketMessage struct {
	Type       int
	Data       []byte
	ReceivedAt time.Time
}

type WebsocketOptions struct {
	Host             string
	Origin           string
	XStreamClient    string
	HandshakeTimeout time.Duration
	PongWait         time.Duration
	PingInterval     time.Duration
	WriteWait        time.Duration
}

type WebsocketSession struct {
	conn *websocket.Conn

	stateMu      sync.RWMutex
	connectionID string
	closed       bool

	writeMu   sync.Mutex
	closeOnce sync.Once

	done      chan struct{}
	err       chan error
	writeWait time.Duration
}

func (s *WebsocketSession) Done() <-chan struct{} {
	return s.done
}

func (s *WebsocketSession) Err() <-chan error {
	return s.err
}

func (s *WebsocketSession) ConnectionID() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.connectionID
}

func (s *WebsocketSession) setConnectionID(id string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.connectionID == "" {
		s.connectionID = id
	}
}

func (s *WebsocketSession) isClosed() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.closed
}

func (s *WebsocketSession) WriteMessage(messageType int, data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.writeWait > 0 {
		_ = s.conn.SetWriteDeadline(time.Now().Add(s.writeWait))
	}
	return s.conn.WriteMessage(messageType, data)
}

func (s *WebsocketSession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.stateMu.Lock()
		s.closed = true
		s.stateMu.Unlock()

		_ = s.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		closeErr = s.conn.Close()
	})
	return closeErr
}

func (c *Client) ConnectWebsocket(ctx context.Context, userID string, messages chan<- WebsocketMessage, opts *WebsocketOptions) (*WebsocketSession, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}
	if c.APIKey == "" {
		return nil, fmt.Errorf("APIKey is required")
	}
	if c.AuthToken == "" {
		return nil, fmt.Errorf("AuthToken is required")
	}
	if messages == nil {
		return nil, fmt.Errorf("messages channel is nil")
	}

	resolved := WebsocketOptions{}
	if opts != nil {
		resolved = *opts
	}
	if resolved.HandshakeTimeout == 0 {
		resolved.HandshakeTimeout = 30 * time.Second
	}
	if resolved.PongWait == 0 {
		resolved.PongWait = 60 * time.Second
	}
	if resolved.PingInterval == 0 {
		resolved.PingInterval = 25 * time.Second
	}
	if resolved.WriteWait == 0 {
		resolved.WriteWait = 10 * time.Second
	}
	if resolved.XStreamClient == "" {
		resolved.XStreamClient = "stream-chat-go-client"
	}

	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse BaseURL: %w", err)
	}
	if base.Scheme == "" && base.Host == "" {
		base, err = url.Parse("https://" + c.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("parse BaseURL: %w", err)
		}
	}

	host := resolved.Host
	if host == "" {
		host = base.Host
	}
	if host == "" {
		return nil, fmt.Errorf("missing host (BaseURL=%q)", c.BaseURL)
	}

	wsScheme := "wss"
	origin := resolved.Origin
	if origin == "" {
		httpScheme := "https"
		if base.Scheme == "http" {
			wsScheme = "ws"
			httpScheme = "http"
		}
		origin = httpScheme + "://" + host
	}

	token := strings.TrimSpace(c.AuthToken)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[len("bearer "):])
	}

	clientRequestID := uuid.NewString()

	payload := struct {
		UserID      string `json:"user_id"`
		UserDetails struct {
			ID string `json:"id"`
		} `json:"user_details"`
		ClientRequestID string `json:"client_request_id"`
	}{
		UserID: userID,
		UserDetails: struct {
			ID string `json:"id"`
		}{
			ID: userID,
		},
		ClientRequestID: clientRequestID,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal connect payload: %w", err)
	}

	q := url.Values{}
	q.Set("json", string(payloadJSON))
	q.Set("api_key", c.APIKey)
	q.Set("authorization", token)
	q.Set("stream-auth-type", "jwt")
	q.Set("X-Stream-Client", resolved.XStreamClient)

	wsURL := url.URL{
		Scheme:   wsScheme,
		Host:     host,
		Path:     "/connect",
		RawQuery: q.Encode(),
	}

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: resolved.HandshakeTimeout,
	}

	headers := http.Header{}
	headers.Set("Origin", origin)

	dialCtx, cancel := context.WithTimeout(ctx, resolved.HandshakeTimeout)
	defer cancel()

	conn, resp, err := dialer.DialContext(dialCtx, wsURL.String(), headers)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, fmt.Errorf("websocket dial failed (http=%d): %w", status, err)
	}

	session := &WebsocketSession{
		conn:      conn,
		done:      make(chan struct{}),
		err:       make(chan error, 1),
		writeWait: resolved.WriteWait,
	}

	session.conn.SetPongHandler(func(string) error {
		return session.conn.SetReadDeadline(time.Now().Add(resolved.PongWait))
	})
	_ = session.conn.SetReadDeadline(time.Now().Add(resolved.PongWait))

	go func() {
		ticker := time.NewTicker(resolved.PingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				_ = session.Close()
				return
			case <-ticker.C:
				if err := session.WriteMessage(websocket.PingMessage, nil); err != nil {
					select {
					case session.err <- fmt.Errorf("websocket ping failed: %w", err):
					default:
					}
					_ = session.Close()
					return
				}
			}
		}
	}()

	go func() {
		defer close(session.done)
		defer func() { _ = session.Close() }()

		for {
			msgType, data, err := session.conn.ReadMessage()
			if err != nil {
				if session.isClosed() || ctx.Err() != nil {
					return
				}
				select {
				case session.err <- err:
				default:
				}
				return
			}

			if msgType == websocket.TextMessage && session.ConnectionID() == "" && json.Valid(data) {
				var tmp struct {
					ConnectionID string `json:"connection_id"`
				}
				if err := json.Unmarshal(data, &tmp); err == nil && tmp.ConnectionID != "" {
					session.setConnectionID(tmp.ConnectionID)
				}
			}

			msg := WebsocketMessage{
				Type:       msgType,
				Data:       append([]byte(nil), data...),
				ReceivedAt: time.Now(),
			}

			select {
			case <-ctx.Done():
				return
			case messages <- msg:
			}
		}
	}()

	return session, nil
}
