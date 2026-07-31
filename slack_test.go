package slack_test

import (
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	contractsnotification "github.com/goravel/framework/contracts/notification"
	mocksclient "github.com/goravel/framework/mocks/http/client"

	"github.com/goravel/slack"
)

// ---- Fakes ----

type slackNotifiable struct{ route any }

// RouteNotificationFor returns any — reconciled against the real
// contracts/notification.Notifiable, which was widened from string.
func (n *slackNotifiable) RouteNotificationFor(channel string) any {
	if channel == "slack" {
		return n.route
	}
	return nil
}

type plainNotification struct{}

func (p *plainNotification) Via(_ contractsnotification.Notifiable) []string {
	return []string{"slack"}
}

type richNotification struct{ msg slack.Message }

func (r *richNotification) Via(_ contractsnotification.Notifiable) []string { return []string{"slack"} }
func (r *richNotification) ToSlack(_ contractsnotification.Notifiable) slack.Message {
	return r.msg
}

// readBody drains the io.Reader passed to Post so tests can assert on
// the actual wire payload sent to chat.postMessage.
func readBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(body)
	assert.NoError(t, err)
	var data map[string]any
	assert.NoError(t, json.Unmarshal(raw, &data))
	return data
}

// ---- Tests ----

func TestChannel_Name(t *testing.T) {
	ch := slack.NewChannel(nil, "xoxb-test")
	assert.Equal(t, "slack", ch.Name())
}

func TestChannel_Send_PostsDefaultMessage_WhenNotSlackNotification(t *testing.T) {
	http := mocksclient.NewFactory(t)
	resp := mocksclient.NewResponse(t)

	var body io.Reader
	http.EXPECT().WithToken("xoxb-test").Return(http).Once()
	http.EXPECT().WithHeader("Content-Type", "application/json; charset=utf-8").Return(http).Once()
	http.EXPECT().Post("https://slack.com/api/chat.postMessage", mock.Anything).
		Run(func(uri string, b io.Reader) {
			body = b
		}).
		Return(resp, nil).Once()

	resp.EXPECT().Successful().Return(true).Once()
	resp.EXPECT().Json().Return(map[string]any{"ok": true}, nil).Once()

	ch := slack.NewChannel(http, "xoxb-test")
	err := ch.Send(&slackNotifiable{route: "#general"}, &plainNotification{})
	assert.NoError(t, err)

	data := readBody(t, body)
	assert.Equal(t, "#general", data["channel"])
	assert.Equal(t, "New notification: *slack_test.plainNotification*", data["text"])
}

func TestChannel_Send_UsesToSlack_WhenSlackNotification(t *testing.T) {
	http := mocksclient.NewFactory(t)
	resp := mocksclient.NewResponse(t)

	var body io.Reader
	http.EXPECT().WithToken("xoxb-test").Return(http).Once()
	http.EXPECT().WithHeader("Content-Type", "application/json; charset=utf-8").Return(http).Once()
	http.EXPECT().Post("https://slack.com/api/chat.postMessage", mock.Anything).
		Run(func(uri string, b io.Reader) {
			body = b
		}).
		Return(resp, nil).Once()

	resp.EXPECT().Successful().Return(true).Once()
	resp.EXPECT().Json().Return(map[string]any{"ok": true}, nil).Once()

	ch := slack.NewChannel(http, "xoxb-test")
	n := &richNotification{msg: slack.Message{Text: "Deploy finished"}}

	err := ch.Send(&slackNotifiable{route: "#alerts"}, n)
	assert.NoError(t, err)

	data := readBody(t, body)
	assert.Equal(t, "#alerts", data["channel"])
	assert.Equal(t, "Deploy finished", data["text"])
}

func TestChannel_Send_ReturnsError_WhenEmptyRoute(t *testing.T) {
	ch := slack.NewChannel(nil, "xoxb-test") // no HTTP calls expected

	err := ch.Send(&slackNotifiable{route: ""}, &plainNotification{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty channel/route")
}

func TestChannel_Send_ReturnsError_WhenRouteIsNil(t *testing.T) {
	// cast.ToString(nil) == "" — same empty-route error path as above,
	// exercised via a notifiable that doesn't route "slack" at all.
	ch := slack.NewChannel(nil, "xoxb-test") // no HTTP calls expected

	err := ch.Send(&slackNotifiable{route: nil}, &plainNotification{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty channel/route")
}

func TestChannel_Deliver_ReturnsError_WhenTokenNotConfigured(t *testing.T) {
	ch := slack.NewChannel(nil, "") // no HTTP calls expected — token check short-circuits first

	payload, err := json.Marshal(slack.Message{Text: "hi"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no bot token configured")
}

func TestChannel_Deliver_ReturnsError_WhenHTTPRequestFails(t *testing.T) {
	http := mocksclient.NewFactory(t)

	http.EXPECT().WithToken("xoxb-test").Return(http).Once()
	http.EXPECT().WithHeader("Content-Type", "application/json; charset=utf-8").Return(http).Once()
	http.EXPECT().Post(mock.Anything, mock.Anything).Return(nil, errors.New("connection refused")).Once()

	ch := slack.NewChannel(http, "xoxb-test")
	payload, err := json.Marshal(slack.Message{Text: "hi"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

// TestChannel_Deliver_ReturnsError_WhenNonSuccessStatus covers a
// genuine HTTP-level failure (e.g. invalid token → 401) — distinct from
// the "ok":false case below, which is Slack's own always-200 failure
// signal.
func TestChannel_Deliver_ReturnsError_WhenNonSuccessStatus(t *testing.T) {
	http := mocksclient.NewFactory(t)
	resp := mocksclient.NewResponse(t)

	http.EXPECT().WithToken("xoxb-test").Return(http).Once()
	http.EXPECT().WithHeader("Content-Type", "application/json; charset=utf-8").Return(http).Once()
	http.EXPECT().Post(mock.Anything, mock.Anything).Return(resp, nil).Once()

	resp.EXPECT().Successful().Return(false).Once()
	resp.EXPECT().Status().Return(401).Once()

	ch := slack.NewChannel(http, "xoxb-test")
	payload, err := json.Marshal(slack.Message{Text: "hi"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// TestChannel_Deliver_ReturnsError_WhenSlackAPIReturnsOkFalse is the
// important one: Slack returns HTTP 200 even on failure — the real
// signal is the "ok" field in the JSON body. This is the exact gotcha
// this package exists to handle correctly.
func TestChannel_Deliver_ReturnsError_WhenSlackAPIReturnsOkFalse(t *testing.T) {
	http := mocksclient.NewFactory(t)
	resp := mocksclient.NewResponse(t)

	http.EXPECT().WithToken("xoxb-test").Return(http).Once()
	http.EXPECT().WithHeader("Content-Type", "application/json; charset=utf-8").Return(http).Once()
	http.EXPECT().Post(mock.Anything, mock.Anything).Return(resp, nil).Once()

	resp.EXPECT().Successful().Return(true).Once() // 200 OK at the HTTP level
	resp.EXPECT().Json().Return(map[string]any{
		"ok":    false,
		"error": "channel_not_found",
	}, nil).Once()

	ch := slack.NewChannel(http, "xoxb-test")
	payload, err := json.Marshal(slack.Message{Text: "hi"})
	assert.NoError(t, err)

	err = ch.Deliver("#nonexistent", payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel_not_found")
}

func TestChannel_Deliver_NoOp_WhenEmptyRoute(t *testing.T) {
	ch := slack.NewChannel(nil, "xoxb-test") // no HTTP calls expected
	err := ch.Deliver("", []byte(`{}`))
	assert.NoError(t, err)
}

func TestChannel_Deliver_ReturnsError_WhenMalformedPayload(t *testing.T) {
	ch := slack.NewChannel(nil, "xoxb-test") // no HTTP calls expected
	err := ch.Deliver("#general", []byte(`{not valid json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}
