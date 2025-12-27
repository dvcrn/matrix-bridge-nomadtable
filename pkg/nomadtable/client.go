package nomadtable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type QueryChannelRequest struct {
	Data     map[string]any `json:"data,omitempty"`
	State    bool           `json:"state"`
	Watch    bool           `json:"watch"`
	Presence bool           `json:"presence"`
}

func (c *Client) QueryChannel(ctx context.Context, channelType, channelID, userID, connectionID string, request *QueryChannelRequest) (*QueryChannelResponse, error) {
	q := url.Values{}
	q.Set("user_id", userID)
	q.Set("connection_id", connectionID)

	path := fmt.Sprintf("/channels/%s/%s/query", channelType, channelID)

	var result QueryChannelResponse
	if err := c.do(ctx, http.MethodPost, path, q, request, &result); err != nil {
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
