package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/status"
)

const (
	LoginFlowIDAuth = "auth"
	LoginStepIDAuth = "auth-input"
	LoginStepIDDone = "complete"
)

type LoginMetadata struct {
	UserID    string `json:"user_id"`
	APIKey    string `json:"api_key"`
	AuthToken string `json:"auth_token"`
}

// SimpleLogin represents an ongoing login attempt.
type SimpleLogin struct {
	User *bridgev2.User
	Main *NomadtableConnector
	Log  zerolog.Logger
}

var _ bridgev2.LoginProcessUserInput = (*SimpleLogin)(nil)

func (sl *SimpleLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	sl.Log.Debug().Msg("Starting auth login flow")
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       LoginStepIDAuth,
		Instructions: "Enter your User ID, API Key, and Auth Token for Nomadtable/Stream.",
		UserInputParams: &bridgev2.LoginUserInputParams{
			Fields: []bridgev2.LoginInputDataField{
				{
					Type: bridgev2.LoginInputFieldTypeUsername,
					ID:   "user_id",
					Name: "User ID",
				},
				{
					Type: bridgev2.LoginInputFieldTypePassword,
					ID:   "api_key",
					Name: "API Key",
				},
				{
					Type: bridgev2.LoginInputFieldTypePassword,
					ID:   "auth_token",
					Name: "Auth Token",
				},
			},
		},
	}, nil
}

func (sl *SimpleLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
	userID := input["user_id"]
	apiKey := input["api_key"]
	authToken := input["auth_token"]

	if userID == "" {
		return nil, fmt.Errorf("user_id cannot be empty")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key cannot be empty")
	}
	if authToken == "" {
		return nil, fmt.Errorf("auth_token cannot be empty")
	}

	sl.Log.Info().Str("user_id", userID).Msg("Received login credentials")

	namespace := uuid.MustParse("f7a4f3e3-5d5a-4a9e-8d8a-3b0b9e8a1b2c")
	loginIDStr := uuid.NewSHA1(namespace, []byte(strings.ToLower(userID))).String()
	loginID := networkid.UserLoginID(loginIDStr)

	ul, err := sl.User.NewLogin(ctx, &database.UserLogin{
		ID:         loginID,
		RemoteName: userID,
		RemoteProfile: status.RemoteProfile{
			Name:     userID,
			Username: userID,
		},
		Metadata: &LoginMetadata{
			UserID:    userID,
			APIKey:    apiKey,
			AuthToken: authToken,
		},
	}, &bridgev2.NewLoginParams{
		DeleteOnConflict: true,
	})
	if err != nil {
		sl.Log.Err(err).Msg("Failed to create user login entry")
		return nil, fmt.Errorf("failed to create user login: %w", err)
	}

	sl.Log.Info().Str("login_id", string(ul.ID)).Msg("Successfully created user login")

	if err = sl.Main.LoadUserLogin(ctx, ul); err != nil {
		sl.Log.Err(err).Msg("Failed to load user login after creation")
	}

	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeComplete,
		StepID:       LoginStepIDDone,
		Instructions: fmt.Sprintf("Successfully logged in as '%s'", userID),
		CompleteParams: &bridgev2.LoginCompleteParams{
			UserLoginID: ul.ID,
			UserLogin:   ul,
		},
	}, nil
}

func (sl *SimpleLogin) Cancel() {
	sl.Log.Debug().Msg("Login process cancelled")
}
