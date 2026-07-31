// Package slack provides a Slack notification channel for Goravel,
// using the Slack Web API (chat.postMessage + bot token) rather than
// Incoming Webhooks — so RouteNotificationFor("slack") can return a
// channel name like "#general" or a user ID for DMs, matching Laravel's
// Slack notification channel, instead of being locked to one fixed
// channel per webhook URL the way Incoming Webhooks force.
package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cast"

	"github.com/goravel/framework/contracts/http/client"
	contractsnotification "github.com/goravel/framework/contracts/notification"
)

const postMessageURL = "https://slack.com/api/chat.postMessage"

// Channel delivers notifications to Slack via chat.postMessage.
//
// No log.Log field — reconciled against the real merged
// notification/channels/{mail,database}.go, which both dropped logging
// entirely (only Manager logs now). Matches that convention rather than
// the earlier draft, which carried a logger like the pre-simplification
// core channels did.
type Channel struct {
	http  client.Factory
	token string
}

// NewChannel constructs the Slack channel. token is the bot token
// (xoxb-...) — see https://api.slack.com/authentication/token-types.
// http is injected (mirrors how MailChannel/DatabaseChannel take their
// dependencies via constructor, not by calling facades.X() internally).
func NewChannel(http client.Factory, token string) *Channel {
	return &Channel{http: http, token: token}
}

func (c *Channel) Name() string { return "slack" }

// Send is the only dispatch method — Channel.SendNow does not exist.
// Reconciled against the real contracts/notification.Channel interface,
// which reverted the SendNow addition from an earlier review round
// entirely (Name() + Send() only). An earlier draft of this file had a
// SendNow method mirroring that now-reverted core pattern; removed.
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
	// RouteNotificationFor returns any, not string — reconciled against
	// the real contracts/notification.Notifiable, which was widened to
	// support per-channel-appropriate types (mail: string/[]string/
	// map[string]string; database: any-castable-to-string via
	// cast.ToString). A Slack channel/user ID is a single string, same
	// shape as database's ID, so this mirrors DatabaseChannel.Resolve's
	// exact cast.ToString(...) pattern rather than inventing a
	// different convention for this channel.
	route := cast.ToString(notifiable.RouteNotificationFor("slack"))
	if route == "" {
		return "", nil, EmptyRoute.Args(notifiable)
	}

	var msg Message
	if sn, ok := n.(Notification); ok {
		msg = sn.ToSlack(notifiable)
	} else {
		msg = c.defaultMessage(n)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return "", nil, MarshalPayload.Args(n, err)
	}

	return route, payload, nil
}

func (c *Channel) Deliver(route string, payload []byte) error {
	if route == "" {
		return nil
	}
	if c.token == "" {
		return TokenNotConfigured
	}

	var msg Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return UnmarshalPayload.Args(err)
	}

	body, err := json.Marshal(wirePayload(route, msg))
	if err != nil {
		return MarshalPayload.Args(msg, err)
	}

	resp, err := c.http.
		WithToken(c.token).
		WithHeader("Content-Type", "application/json; charset=utf-8").
		Post(postMessageURL, bytes.NewReader(body))
	if err != nil {
		return RequestFailed.Args(err)
	}

	// Slack's Web API returns HTTP 200 even when the request fails at
	// the application level — the real failure signal is the "ok"
	// field in the JSON body, not the status code. Still worth checking
	// Successful() first for genuine HTTP-level failures (auth/network
	// issues, rate limiting) that wouldn't even have a well-formed body.
	if !resp.Successful() {
		return NonSuccessStatus.Args(resp.Status())
	}

	data, err := resp.Json()
	if err != nil {
		return DecodeResponse.Args(err)
	}

	if ok, _ := data["ok"].(bool); !ok {
		apiErr, _ := data["error"].(string)
		return APIError.Args(apiErr)
	}

	return nil
}

func (c *Channel) defaultMessage(n contractsnotification.Notification) Message {
	typeName := strings.TrimPrefix(fmt.Sprintf("%T", n), "*")
	return Message{Text: fmt.Sprintf("New notification: *%s*", typeName)}
}

// ---- internal wire types for chat.postMessage ----

type slackWirePayload struct {
	Channel     string                `json:"channel"`
	Text        string                `json:"text"`
	ThreadTS    string                `json:"thread_ts,omitempty"`
	Attachments []slackWireAttachment `json:"attachments,omitempty"`
	Blocks      json.RawMessage       `json:"blocks,omitempty"`
}

type slackWireAttachment struct {
	Title  string           `json:"title,omitempty"`
	Text   string           `json:"text,omitempty"`
	Color  string           `json:"color,omitempty"`
	Fields []slackWireField `json:"fields,omitempty"`
	Footer string           `json:"footer,omitempty"`
	Ts     int64            `json:"ts,omitempty"`
}

type slackWireField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func wirePayload(route string, msg Message) slackWirePayload {
	p := slackWirePayload{
		Channel:  route,
		Text:     msg.Text,
		ThreadTS: msg.ThreadTS,
		Blocks:   json.RawMessage(msg.Blocks),
	}
	for _, a := range msg.Attachments {
		wa := slackWireAttachment{
			Title:  a.Title,
			Text:   a.Text,
			Color:  a.Color,
			Footer: a.Footer,
			Ts:     a.Timestamp,
		}
		for _, f := range a.Fields {
			wa.Fields = append(wa.Fields, slackWireField(f))
		}
		p.Attachments = append(p.Attachments, wa)
	}
	return p
}

var (
	_ contractsnotification.Channel           = (*Channel)(nil)
	_ contractsnotification.ResolvableChannel = (*Channel)(nil)
)
