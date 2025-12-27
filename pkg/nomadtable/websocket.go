package nomadtable

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

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
	HealthInterval   time.Duration
	WriteWait        time.Duration

	// Logger is optional. Called on receive/send with a short message.
	Logger func(format string, args ...any)
}

type WebsocketSession struct {
	conn *websocket.Conn
	log  func(format string, args ...any)

	stateMu      sync.RWMutex
	connectionID string
	clientID     string
	closed       bool

	writeMu   sync.Mutex
	closeOnce sync.Once

	done      chan struct{}
	err       chan error
	writeWait time.Duration
}

func (s *WebsocketSession) Done() <-chan struct{} { return s.done }

func (s *WebsocketSession) Err() <-chan error { return s.err }

func (s *WebsocketSession) ConnectionID() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.connectionID
}

func (s *WebsocketSession) ClientID() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.clientID
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
	s.log("ws send type=%d bytes=%d", messageType, len(data))
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
	if resolved.HealthInterval == 0 {
		resolved.HealthInterval = 30 * time.Second
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
	clientID := userID + "--" + clientRequestID

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

	logFn := resolved.Logger
	if logFn == nil {
		logFn = func(string, ...any) {}
	}

	session := &WebsocketSession{
		conn:      conn,
		log:       logFn,
		done:      make(chan struct{}),
		err:       make(chan error, 1),
		writeWait: resolved.WriteWait,
		clientID:  clientID,
	}

	session.conn.SetPingHandler(func(appData string) error {
		// Gorilla does not automatically respond to ping frames.
		session.log("ws recv ping bytes=%d", len(appData))
		_ = session.WriteMessage(websocket.PongMessage, []byte(appData))
		return nil
	})
	session.conn.SetPongHandler(func(appData string) error {
		session.log("ws recv pong bytes=%d", len(appData))
		return session.conn.SetReadDeadline(time.Now().Add(resolved.PongWait))
	})
	_ = session.conn.SetReadDeadline(time.Now().Add(resolved.PongWait))

	// Keep the underlying websocket alive.
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

	// Stream expects the client to send application-level health checks.
	// The server replies with a `health.check` event.
	go func() {
		ticker := time.NewTicker(resolved.HealthInterval)
		defer ticker.Stop()

		payload := []map[string]string{{
			"type":      "health.check",
			"client_id": session.ClientID(),
		}}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			select {
			case session.err <- fmt.Errorf("marshal health.check: %w", err):
			default:
			}
			_ = session.Close()
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				session.log("ws send health.check client_id=%s", session.ClientID())
				if err := session.WriteMessage(websocket.TextMessage, payloadJSON); err != nil {
					select {
					case session.err <- fmt.Errorf("send health.check: %w", err):
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
			if err == nil {
				session.log("ws recv type=%d bytes=%d", msgType, len(data))
			}
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

			if msgType == websocket.TextMessage && json.Valid(data) {
				// Fast-path: capture connection_id if present.
				if session.ConnectionID() == "" {
					var tmp struct {
						ConnectionID string `json:"connection_id"`
					}
					if err := json.Unmarshal(data, &tmp); err == nil && tmp.ConnectionID != "" {
						session.setConnectionID(tmp.ConnectionID)
					}
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
