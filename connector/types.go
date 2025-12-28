package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/dvcrn/matrix-bridge-nomadtable/pkg/nomadtable"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"
)

type NomadtableMessageMetadata struct {
	CID         string `json:"cid,omitempty"`
	MessageType string `json:"message_type,omitempty"`
	UserID      string `json:"user_id,omitempty"`
}

type NomadtableRemoteMessage struct {
	simplevent.EventMeta
	Msg       *nomadtable.Message
	MessageID networkid.MessageID
}

var _ bridgev2.RemoteMessage = (*NomadtableRemoteMessage)(nil)

func (m *NomadtableRemoteMessage) GetID() networkid.MessageID {
	return m.MessageID
}

func (m *NomadtableRemoteMessage) ConvertMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI) (*bridgev2.ConvertedMessage, error) {
	_ = portal
	_ = intent

	if m.Msg == nil {
		return nil, fmt.Errorf("missing message")
	}

	body := m.Msg.Text
	formatted := ""
	if m.Msg.HTML != "" {
		formatted = m.Msg.HTML
		if body == "" {
			body = m.Msg.HTML
		}
	}
	if body == "" {
		body = "(empty message)"
	}

	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
	}
	if formatted != "" {
		content.Format = event.FormatHTML
		content.FormattedBody = formatted
	}

	part := &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: content,
		DBMetadata: &NomadtableMessageMetadata{
			CID:         string(m.PortalKey.ID),
			MessageType: m.Msg.Type,
			UserID:      "",
		},
	}
	if m.Msg.User != nil {
		part.DBMetadata.(*NomadtableMessageMetadata).UserID = m.Msg.User.ID
	}

	return &bridgev2.ConvertedMessage{Parts: []*bridgev2.ConvertedMessagePart{part}}, nil
}

func parsePortalKeyToChannel(portalID networkid.PortalID) (channelType, channelID string, err error) {
	raw := string(portalID)
	if raw == "" {
		return "", "", fmt.Errorf("empty portal id")
	}

	// We store portals as Stream CIDs: "type:id".
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unsupported portal id format: %q", raw)
	}

	return parts[0], parts[1], nil
}
