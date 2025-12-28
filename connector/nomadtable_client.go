package connector

import (
	"context"

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
}

// Connect is a no-op for this simple connector.
func (nc *NomadtableClient) Connect(ctx context.Context) {
	nc.log.Info().Msg("NomadtableClient Connect called")
}

func (nc *NomadtableClient) Disconnect() {
	nc.log.Info().Msg("NomadtableClient Disconnect called")
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
