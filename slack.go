// Package slack provides a Slack notification channel for Goravel,
// using the Slack Web API (chat.postMessage + a bot token, via
// github.com/slack-go/slack) rather than Incoming Webhooks — so
// RouteNotificationFor("slack") can return a channel name like
// "#general" or a user ID for a DM, instead of being locked to one
// fixed channel per webhook URL the way Incoming Webhooks force.
//
// Uses slack-go/slack rather than a hand-rolled HTTP client — it's the
// de facto standard Go Slack SDK, matches the pattern goravel/redis and
// similar driver packages already use (wrap the established Go client
// for the service, don't reimplement its wire protocol), and it already
// handles Slack's "always HTTP 200, check the ok field" failure signal
// internally — every real slack-go/slack example just checks
// `if err != nil`, so this package no longer needs its own
// Successful()/Json()/"ok" parsing at all.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/slack-go/slack"
	"github.com/spf13/cast"

	contractsnotification "github.com/goravel/framework/contracts/notification"

	"github.com/goravel/slack/contracts"
)

// defaultTimeout bounds every chat.postMessage call. Without a
// deadline, a network partition or a slow/hanging Slack API response
// blocks the calling goroutine indefinitely — worse under a queued
// worker, where a stuck goroutine holds up other queued deliveries
// too. slack-go/slack's PostMessageContext is what makes this
// enforceable; the plain PostMessage variant has no way to bound it.
var defaultTimeout = 30 * time.Second

// Channel delivers notifications to Slack via chat.postMessage.
type Channel struct {
	client *slack.Client
	token  string // kept alongside client for the empty-token fast-fail check in Deliver
}

// NewChannel constructs the Slack channel. token is the bot token
// (xoxb-...) — see https://api.slack.com/authentication/token-types.
//
// No separate *http.Client parameter — an earlier version had one
// alongside opts, which was redundant (every real call site passed nil,
// and tests use OptionAPIURL in opts, not a custom client). Pass
// slack.OptionHTTPClient(client) in opts directly if a custom HTTP
// client is needed; app.MakeHttp() returns contracts/http.Http, not
// *http.Client, so it couldn't have substituted for this parameter
// anyway.
func NewChannel(token string, opts ...slack.Option) *Channel {
	return &Channel{client: slack.New(token, opts...), token: token}
}

func (c *Channel) Name() string { return "slack" }

func (c *Channel) Send(
	notifiable contractsnotification.Notifiable,
	n contractsnotification.Notification,
) error {
	route, payload, err := c.Resolve(notifiable, n)
	if err != nil {
		return err
	}
	return c.Deliver(route, payload)
}

func (c *Channel) Resolve(
	notifiable contractsnotification.Notifiable,
	n contractsnotification.Notification,
) (string, []byte, error) {
	// RouteNotificationFor returns any (mail: string/[]string/
	// map[string]string, database: any-castable-to-string, custom: any
	// per-channel shape). A Slack channel/user ID is a single string,
	// same shape as database's ID, so this mirrors DatabaseChannel's
	// cast.ToString(...) pattern.
	route := cast.ToString(notifiable.RouteNotificationFor("slack"))
	if route == "" {
		return "", nil, ErrorEmptyRoute.Args(notifiable)
	}

	var msg contracts.Message
	if sn, ok := n.(contracts.Notification); ok {
		msg = sn.ToSlack(notifiable)
	} else {
		msg = c.defaultMessage(n)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return "", nil, ErrorMarshalPayload.Args(n, err)
	}

	return route, payload, nil
}

func (c *Channel) Deliver(route string, payload []byte) error {
	if route == "" {
		return nil
	}
	if c.token == "" {
		return ErrorTokenNotConfigured
	}

	var msg contracts.Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return ErrorUnmarshalPayload.Args(err)
	}

	options := make([]slack.MsgOption, 0, 3) // text + attachments + thread_ts, the common case
	options = append(options, slack.MsgOptionText(msg.Text, false))

	if len(msg.Attachments) > 0 {
		options = append(options, slack.MsgOptionAttachments(toSDKAttachments(msg.Attachments)...))
	}
	if msg.ThreadTS != "" {
		options = append(options, slack.MsgOptionTS(msg.ThreadTS))
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// PostMessageContext returns a proper Go error for Slack's
	// "ok": false responses already — no manual status-code/body-field
	// checking needed, unlike the raw-HTTP version this replaced.
	if _, _, err := c.client.PostMessageContext(ctx, route, options...); err != nil {
		return ErrorPostMessageFailed.Args(err)
	}

	return nil
}

func (c *Channel) defaultMessage(n contractsnotification.Notification) contracts.Message {
	// %T alone would put a raw Go type path (e.g.
	// "*myservice.InvoicePaid") directly into a Slack message visible
	// to end users — fine as a log line, not as something a human
	// reads in a channel. Matches the phrasing MailChannel's own
	// default message uses ("You have a new %T notification.") rather
	// than exposing the bare type.
	return contracts.Message{Text: fmt.Sprintf("You have a new %T notification.", n)}
}

// toSDKAttachments maps our own Attachment type to slack-go/slack's.
// Timestamp maps to Ts json.Number — confirmed directly against the
// real slack-go/slack source (attachments.go: `Ts json.Number
// `json:"ts,omitempty"“), not guessed; json.Number is just a string
// underneath, hence the strconv.FormatInt conversion below.
func toSDKAttachments(attachments []contracts.Attachment) []slack.Attachment {
	out := make([]slack.Attachment, 0, len(attachments))
	for i := range attachments {
		a := &attachments[i] // avoid copying the struct on each iteration
		sa := slack.Attachment{
			Title:  a.Title,
			Text:   a.Text,
			Color:  a.Color,
			Footer: a.Footer,
			Fields: make([]slack.AttachmentField, 0, len(a.Fields)),
		}
		if a.Timestamp > 0 {
			sa.Ts = json.Number(strconv.FormatInt(a.Timestamp, 10))
		}
		for _, f := range a.Fields {
			sa.Fields = append(sa.Fields, slack.AttachmentField{
				Title: f.Title,
				Value: f.Value,
				Short: f.Short,
			})
		}
		out = append(out, sa)
	}
	return out
}

var (
	_ contractsnotification.Channel           = (*Channel)(nil)
	_ contractsnotification.ResolvableChannel = (*Channel)(nil)
)
