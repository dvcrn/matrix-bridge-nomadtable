package connector

import (
	"context"
	"fmt"

	"github.com/dvcrn/matrix-bridge-nomadtable/pkg/nomadtable"
	"github.com/rs/zerolog"
	"go.mau.fi/util/configupgrade"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
)

// Ensure MyConnector implements NetworkConnector.
var _ bridgev2.NetworkConnector = (*NomadtableConnector)(nil)

// NomadtableConnector implements the NetworkConnector interface.
type NomadtableConnector struct {
	log    zerolog.Logger
	bridge *bridgev2.Bridge
}

// NewMyConnector creates a new instance of MyConnector.
func NewMyConnector(log zerolog.Logger) *NomadtableConnector {
	return &NomadtableConnector{
		log: log.With().Str("component", "network-connector").Logger(),
	}
}

// Init initializes the connector with the bridge instance.
func (c *NomadtableConnector) Init(br *bridgev2.Bridge) {
	c.bridge = br
	c.log = c.bridge.Log
	c.log.Info().Msg("MyConnector Init called")
}

// GetName implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) GetName() bridgev2.BridgeName {
	return bridgev2.BridgeName{
		DisplayName:          "Simple Bridge",
		NetworkURL:           "https://example.org",
		NetworkIcon:          "",
		NetworkID:            "simplenetwork",
		BeeperBridgeType:     "simple",
		DefaultPort:          29319,
		DefaultCommandPrefix: "!simple",
	}
}

// GetNetworkID implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) GetNetworkID() string {
	return c.GetName().NetworkID
}

// GetCapabilities implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{}
}

// GetDBMetaTypes implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Message:   func() any { return &NomadtableMessageMetadata{} },
		UserLogin: func() any { return &LoginMetadata{} },
	}
}

// GetLoginFlows implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return []bridgev2.LoginFlow{{
		ID:          LoginFlowIDAuth,
		Name:        "User ID & Auth Token",
		Description: "Log in using a User ID, API Key, and Auth Token.",
	}}
}

// CreateLogin implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) CreateLogin(ctx context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	if flowID != LoginFlowIDAuth {
		return nil, fmt.Errorf("unsupported login flow ID: %s", flowID)
	}
	return &SimpleLogin{
		User: user,
		Main: c,
		Log:  user.Log.With().Str("action", "login").Str("flow", flowID).Logger(),
	}, nil
}

// GetConfig implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) GetConfig() (string, any, configupgrade.Upgrader) {
	return "simple-config.yaml", nil, nil
}

// GetBridgeInfoVersion implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) GetBridgeInfoVersion() (int, int) {
	return 1, 0
}

// Start implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) Start(ctx context.Context) error {
	c.log.Info().Msg("MyConnector Start called")
	return nil
}

// Stop implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) Stop(ctx context.Context) error {
	c.log.Info().Msg("MyConnector Stop called")
	return nil
}

// LoadUserLogin implements bridgev2.NetworkConnector.
func (c *NomadtableConnector) LoadUserLogin(ctx context.Context, login *bridgev2.UserLogin) error {
	c.log.Info().
		Str("user_id", string(login.ID)).
		Str("remote_name", login.RemoteName).
		Str("mxid", string(login.User.MXID)).
		Msg("LoadUserLogin called")

	meta, ok := login.Metadata.(*LoginMetadata)
	if !ok {
		return fmt.Errorf("invalid login metadata type")
	}

	updatedProfile := false
	if login.RemoteName == "" && meta.UserID != "" {
		login.RemoteName = meta.UserID
		updatedProfile = true
	}
	if login.RemoteProfile.Name == "" && meta.UserID != "" {
		login.RemoteProfile.Name = login.RemoteName
		updatedProfile = true
	}
	if login.RemoteProfile.Username == "" && meta.UserID != "" {
		login.RemoteProfile.Username = meta.UserID
		updatedProfile = true
	}
	if updatedProfile {
		if err := login.Save(ctx); err != nil {
			c.log.Err(err).
				Str("user_login_id", string(login.ID)).
				Msg("Failed to update user login profile")
		}
	}

	nc := nomadtable.NewClient(meta.APIKey, meta.AuthToken)

	client := &NomadtableClient{
		log:       c.log.With().Str("user_id", string(login.ID)).Logger(),
		bridge:    c.bridge,
		login:     login,
		connector: c,
		meta:      meta,
		client:    nc,
	}

	login.Client = client

	c.log.Info().
		Str("user_id", string(login.ID)).
		Str("remote_name", login.RemoteName).
		Interface("client_type", client).
		Msg("Created and stored NomadtableClient")

	return nil
}
