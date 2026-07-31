package slack

import "github.com/goravel/framework/errors"

var (
	EmptyRoute         = errors.New("slack channel: %T.RouteNotificationFor(\"slack\") returned empty channel/route").SetModule(Module)
	MarshalPayload     = errors.New("slack channel: failed to marshal payload for %T: %w").SetModule(Module)
	UnmarshalPayload   = errors.New("slack channel: failed to unmarshal payload: %w").SetModule(Module)
	RequestFailed      = errors.New("slack channel: request to chat.postMessage failed: %w").SetModule(Module)
	NonSuccessStatus   = errors.New("slack channel: chat.postMessage returned non-success status %d").SetModule(Module)
	DecodeResponse     = errors.New("slack channel: failed to decode chat.postMessage response: %w").SetModule(Module)
	APIError           = errors.New("slack channel: chat.postMessage returned ok=false, error=%q").SetModule(Module)
	TokenNotConfigured = errors.New("slack channel: no bot token configured (set SLACK_BOT_TOKEN or slack.token)").SetModule(Module)
)

const Module = "slack"
