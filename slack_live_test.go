package slack_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	mockslog "github.com/goravel/framework/mocks/log"
	"github.com/goravel/framework/notification"

	"github.com/goravel/slack"
	"github.com/goravel/slack/contracts"
)

// TestLiveSlackChannel sends a real Slack message through the framework's
// notification manager — the same full path an app uses, rather than
// calling the channel directly. Mirrors the framework's own live SMTP
// suite (framework/mail/application_test.go): skipped locally unless the
// env vars are set, exercised for real in CI via .github/workflows/slack.yml.
//
// No explicit HTTP timeout configured here — Channel.Deliver already
// wraps every PostMessageContext call in a context.WithTimeout(30s)
// internally (see slack.go), so this test inherits that bound rather
// than needing its own.
func TestLiveSlackChannel(t *testing.T) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		t.Skip("SLACK_BOT_TOKEN/SLACK_CHANNEL not set — skipping live Slack delivery test")
	}

	logger := mockslog.NewLog(t)
	// On delivery failure the notification manager logs via Errorf before
	// returning the error. Without this permissive expectation, that would
	// trip testify's "unexpected call" guard and mask the real Slack error
	// (e.g. not_in_channel). Success path never calls Errorf. First arg
	// (the log format string) tightened to AnythingOfType("string") rather
	// than a fully loose mock.Anything, per review — Errorf's signature is
	// Errorf(format string, args ...any), so this asserts the actual
	// contract rather than accepting any type in that position.
	logger.EXPECT().
		Errorf(mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything).
		Return().Maybe()

	manager := notification.NewManager(logger, nil)
	manager.Extend(slack.NewChannel(token))

	n := &richNotification{msg: contracts.Message{Text: "Goravel Slack integration test — live delivery"}}

	err := manager.Route("slack", channel).NotifyNow(n)
	assert.NoError(t, err)
}
