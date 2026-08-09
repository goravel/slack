package slack_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	slackgo "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"

	contractsnotification "github.com/goravel/framework/contracts/notification"

	"github.com/goravel/slack"
)

// ---- Fakes ----

type slackNotifiable struct{ route any }

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

// ---- Test server helpers ----
//
// slack-go/slack's PostMessage sends application/x-www-form-urlencoded
// data (confirmed against the real slack-go/slack test suite, e.g.
// chat_test.go — every handler there uses r.ParseForm(), not JSON
// decoding), and OptionAPIURL(u) redirects the client at a given base
// URL — also confirmed directly from slack-go/slack's own source
// (slack.go: "OptionAPIURL set the url for the client. only useful for
// testing.").

// newTestServer starts an httptest.Server whose /chat.postMessage
// handler responds however respond specifies, and returns a Channel
// pointed at it plus a func to read back the last request's form values.
func newTestServer(t *testing.T, respond http.HandlerFunc) (*slack.Channel, func() map[string][]string, func()) {
	t.Helper()

	var lastForm map[string][]string
	mux := http.NewServeMux()
	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())
		lastForm = map[string][]string(r.Form)
		respond(w, r)
	})

	server := httptest.NewServer(mux)
	ch := slack.NewChannel("xoxb-test", nil, slackgo.OptionAPIURL(server.URL+"/"))

	return ch, func() map[string][]string { return lastForm }, server.Close
}

func okResponse(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok": true, "channel": "C123", "ts": "1234567890.123456"}`))
}

func errorResponse(errCode string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": false, "error": "` + errCode + `"}`))
	}
}

// ---- Tests ----

func TestChannel_Name(t *testing.T) {
	ch := slack.NewChannel("xoxb-test", nil)
	assert.Equal(t, "slack", ch.Name())
}

func TestChannel_Send_PostsDefaultMessage_WhenNotSlackNotification(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	err := ch.Send(&slackNotifiable{route: "#general"}, &plainNotification{})
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "#general", form["channel"][0])
	assert.Contains(t, form["text"][0], "slack_test.plainNotification")
}

func TestChannel_Send_UsesToSlack_WhenSlackNotification(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	n := &richNotification{msg: slack.Message{Text: "Deploy finished"}}
	err := ch.Send(&slackNotifiable{route: "#alerts"}, n)
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "#alerts", form["channel"][0])
	assert.Equal(t, "Deploy finished", form["text"][0])
}

func TestChannel_Send_ReturnsError_WhenEmptyRoute(t *testing.T) {
	ch := slack.NewChannel("xoxb-test", nil) // no server needed — should never make a request

	err := ch.Send(&slackNotifiable{route: ""}, &plainNotification{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty channel/route")
}

func TestChannel_Send_ReturnsError_WhenRouteIsNil(t *testing.T) {
	ch := slack.NewChannel("xoxb-test", nil) // no server needed

	err := ch.Send(&slackNotifiable{route: nil}, &plainNotification{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty channel/route")
}

func TestChannel_Deliver_ReturnsError_WhenTokenNotConfigured(t *testing.T) {
	ch := slack.NewChannel("", nil) // no server needed — token check short-circuits first

	payload, err := json.Marshal(slack.Message{Text: "hi"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no bot token configured")
}

// TestChannel_Deliver_ReturnsError_WhenSlackAPIReturnsOkFalse confirms
// slack-go/slack surfaces Slack's "ok": false failures as a proper Go
// error automatically — no manual status-code/body-field parsing needed
// on our side, unlike the raw-HTTP version this replaced. Confirmed
// against slack-go/slack's own TestPostMessageInvalidChannel: err.Error()
// is the raw Slack error code, unwrapped.
func TestChannel_Deliver_ReturnsError_WhenSlackAPIReturnsOkFalse(t *testing.T) {
	ch, _, closeServer := newTestServer(t, errorResponse("channel_not_found"))
	defer closeServer()

	payload, err := json.Marshal(slack.Message{Text: "hi"})
	assert.NoError(t, err)

	err = ch.Deliver("#nonexistent", payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel_not_found")
}

func TestChannel_Deliver_NoOp_WhenEmptyRoute(t *testing.T) {
	ch := slack.NewChannel("xoxb-test", nil) // no server needed
	err := ch.Deliver("", []byte(`{}`))
	assert.NoError(t, err)
}

func TestChannel_Deliver_ReturnsError_WhenMalformedPayload(t *testing.T) {
	ch := slack.NewChannel("xoxb-test", nil) // no server needed
	err := ch.Deliver("#general", []byte(`{not valid json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestChannel_Deliver_SendsAttachments(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(slack.Message{
		Text: "Invoice paid",
		Attachments: []slack.Attachment{
			{Title: "Details", Color: "good", Fields: []slack.Field{
				{Title: "Amount", Value: "$99.00", Short: true},
			}},
		},
	})
	assert.NoError(t, err)

	err = ch.Deliver("#billing", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Contains(t, form, "attachments")
}

// TestChannel_Deliver_MapsAttachmentTimestamp confirms the
// Attachment.Timestamp → slack.Attachment.Ts (json.Number) conversion —
// confirmed directly against the real slack-go/slack source
// (attachments.go: `Ts json.Number`), not guessed. Attachments are sent
// as a single JSON-encoded form field ("attachments"), so this decodes
// that field back out to check the ts value round-tripped correctly as
// a number, not a string that happens to look like one.
func TestChannel_Deliver_MapsAttachmentTimestamp(t *testing.T) {
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(slack.Message{
		Text: "Deploy finished",
		Attachments: []slack.Attachment{
			{Title: "Details", Timestamp: 1700000000},
		},
	})
	assert.NoError(t, err)

	err = ch.Deliver("#deploys", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Contains(t, form, "attachments")

	var decoded []map[string]any
	assert.NoError(t, json.Unmarshal([]byte(form["attachments"][0]), &decoded))
	assert.Len(t, decoded, 1)
	// json.Number round-trips through JSON as a bare number, not a
	// quoted string — confirms the mapping used the right type.
	assert.EqualValues(t, 1700000000, decoded[0]["ts"])
}

func TestChannel_Deliver_SendsThreadTS(t *testing.T) {
	// "thread_ts" confirmed directly against slack-go/slack's real
	// source (chat.go: MsgOptionTS does config.values.Set("thread_ts", ts)).
	ch, lastForm, closeServer := newTestServer(t, okResponse)
	defer closeServer()

	payload, err := json.Marshal(slack.Message{Text: "reply", ThreadTS: "1234.5678"})
	assert.NoError(t, err)

	err = ch.Deliver("#general", payload)
	assert.NoError(t, err)

	form := lastForm()
	assert.Equal(t, "1234.5678", form["thread_ts"][0])
}
