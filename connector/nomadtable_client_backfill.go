package connector

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/dvcrn/matrix-bridge-nomadtable/pkg/nomadtable"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
)

// BackfillingNetworkAPI is responsible for loading historic messages.
var _ bridgev2.BackfillingNetworkAPI = (*NomadtableClient)(nil)

// FetchMessages implements [bridgev2.BackfillingNetworkAPI].
func (nc *NomadtableClient) FetchMessages(ctx context.Context, fetchParams bridgev2.FetchMessagesParams) (*bridgev2.FetchMessagesResponse, error) {
	fmt.Println("----------")
	fmt.Println("----------")
	fmt.Println("----------")
	fmt.Println("----------")
	fmt.Println("---------- Backfill called")
	portal := fetchParams.Portal
	log := nc.log.With().
		Str("portal_id", string(portal.ID)).
		Str("portal_mxid", string(portal.MXID)).
		Bool("forward", fetchParams.Forward).
		Int("count", fetchParams.Count).
		Logger()
	ctx = log.WithContext(ctx)
	log.Info().Msg("FetchMessages called")

	if nc.session == nil {
		return nil, fmt.Errorf("no websocket session")
	}
	connectionID := nc.session.ConnectionID()
	if connectionID == "" {
		return nil, fmt.Errorf("missing connection_id")
	}

	channelType, channelID, err := parsePortalKeyToChannel(portal.PortalKey.ID)
	if err != nil {
		return nil, err
	}

	resp, err := nc.client.QueryChannel(ctx, channelType, channelID, nc.meta.UserID, connectionID, &nomadtable.QueryChannelRequest{
		State:    true,
		Watch:    false,
		Presence: false,
	})
	if err != nil {
		return nil, fmt.Errorf("QueryChannel failed: %w", err)
	}

	msgs := resp.Messages
	sort.Slice(msgs, func(i, j int) bool {
		mi := msgs[i]
		mj := msgs[j]
		if mi == nil {
			return false
		}
		if mj == nil {
			return true
		}
		if mi.CreatedAt == nil {
			return false
		}
		if mj.CreatedAt == nil {
			return true
		}
		return mi.CreatedAt.Before(*mj.CreatedAt)
	})

	limit := fetchParams.Count
	if limit <= 0 {
		limit = 30
	}
	if limit > len(msgs) {
		limit = len(msgs)
	}

	// Backfill batches should be chronological.
	batch := msgs
	// Without pagination, return the most recent messages for both directions.
	if len(batch) > limit {
		batch = batch[len(batch)-limit:]
	}

	out := make([]*bridgev2.BackfillMessage, 0, len(batch))
	for _, m := range batch {
		if m == nil {
			continue
		}

		ts := time.Now()
		if m.CreatedAt != nil {
			ts = *m.CreatedAt
		}

		senderID := "example-ghost"
		if m.User != nil {
			if m.User.ID != "" {
				senderID = m.User.ID
			}
		}

		body := m.Text
		formatted := ""
		if m.HTML != "" {
			formatted = m.HTML
			if body == "" {
				body = m.HTML
			}
		}
		if body == "" {
			body = "(empty message)"
		}

		content := &event.MessageEventContent{MsgType: event.MsgText, Body: body}
		if formatted != "" {
			content.Format = event.FormatHTML
			content.FormattedBody = formatted
		}

		var replyTo *networkid.MessageOptionalPartID
		quotedID := m.QuotedMessageID
		if quotedID == "" && m.QuotedMessage != nil {
			quotedID = m.QuotedMessage.ID
		}
		if quotedID != "" {
			replyTo = &networkid.MessageOptionalPartID{
				MessageID: networkid.MessageID(quotedID),
			}
		}

		out = append(out, &bridgev2.BackfillMessage{
			ID:        networkid.MessageID(m.ID),
			Timestamp: ts,
			Sender: bridgev2.EventSender{
				Sender:   networkid.UserID(senderID),
				IsFromMe: nc.IsThisUser(ctx, networkid.UserID(senderID)),
			},
			ConvertedMessage: &bridgev2.ConvertedMessage{
				ReplyTo: replyTo,
				Parts: []*bridgev2.ConvertedMessagePart{{
					Type:    event.EventMessage,
					Content: content,
					DBMetadata: &NomadtableMessageMetadata{
						CID:         m.CID,
						MessageType: m.Type,
						UserID:      senderID,
					},
				}},
			},
		})
	}

	return &bridgev2.FetchMessagesResponse{
		Messages:                out,
		HasMore:                 false,
		Forward:                 fetchParams.Forward,
		MarkRead:                !fetchParams.Forward,
		AggressiveDeduplication: true,
	}, nil
}
