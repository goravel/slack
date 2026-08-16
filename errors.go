package slack

import "github.com/goravel/framework/errors"

var (
	ErrorEmptyRoute         = errors.New("slack channel: %T.RouteNotificationFor(\"slack\") returned empty channel/route").SetModule(Module)
	ErrorMarshalPayload     = errors.New("slack channel: failed to marshal payload for %T: %v").SetModule(Module)
	ErrorUnmarshalPayload   = errors.New("slack channel: failed to unmarshal payload: %v").SetModule(Module)
	ErrorPostMessageFailed  = errors.New("slack channel: chat.postMessage failed: %v").SetModule(Module)
	ErrorTokenNotConfigured = errors.New("slack channel: no bot token configured (set SLACK_BOT_TOKEN or slack.token)").SetModule(Module)
)

const Module = "slack"
