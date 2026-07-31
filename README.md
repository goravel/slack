<div align="center">

[![Doc](https://pkg.go.dev/badge/github.com/goravel/slack)](https://pkg.go.dev/github.com/goravel/slack)
[![Go](https://img.shields.io/github/go-mod/go-version/goravel/slack)](https://go.dev/)
[![Release](https://img.shields.io/github/release/goravel/slack.svg)](https://github.com/goravel/slack/releases)
[![Test](https://github.com/goravel/slack/actions/workflows/test.yml/badge.svg)](https://github.com/goravel/slack/actions)
[![Report Card](https://goreportcard.com/badge/github.com/goravel/slack)](https://goreportcard.com/report/github.com/goravel/slack)
[![Codecov](https://codecov.io/gh/goravel/slack/branch/master/graph/badge.svg)](https://codecov.io/gh/goravel/slack)
![License](https://img.shields.io/github/license/goravel/slack)

</div>

A Slack notification channel for [Goravel](https://github.com/goravel/framework)'s
[notification module](https://www.goravel.dev/digging-deeper/notifications.html),
using the Slack Web API (`chat.postMessage` + a bot token) instead of
Incoming Webhooks — so `RouteNotificationFor("slack")` can return a
channel name like `#general` or a user ID for a DM, rather than being
locked to whichever single channel an Incoming Webhook was created for.

## Version

| goravel/slack | goravel/framework |
|----------------|--------------------|
| v1.0.*         | v1.18.*            |

## Install

Run the command below in your project to install the package
automatically:

```shell
./artisan package:install github.com/goravel/slack
```

Or check the [setup file](./setup/setup.go) to install the package
manually.

## Configuration

Publish the config file, then set your bot token:

```shell
./artisan vendor:publish
```

This writes `config/slack.go` into your app (`package:install` only
wires the service provider — publishing config is the separate step
Goravel packages use for this, matching `app.Publishes(...)`'s
documented pattern). Then in `.env`:

```
SLACK_BOT_TOKEN=xoxb-your-token-here
```

Create a bot at [api.slack.com/apps](https://api.slack.com/apps) with
the `chat:write` scope, then invite it to any channel it needs to post
in — a bot can only post to channels it's been added to.

The published config:

```go
package config

import "github.com/goravel/framework/facades"

func init() {
	facades.Config().Add("slack", map[string]any{
		"token": facades.Config().Env("SLACK_BOT_TOKEN", ""),
	})
}
```

## Usage

Implement `slack.Notification` on any notification and add `"slack"` to
its `Via()` list:

```go
package notifications

import (
	"github.com/goravel/framework/contracts/notification"
	"github.com/goravel/slack"
)

type InvoicePaid struct {
	Invoice *models.Invoice
}

func (n *InvoicePaid) Via(notifiable notification.Notifiable) []string {
	return []string{"slack"}
}

func (n *InvoicePaid) ToSlack(notifiable notification.Notifiable) slack.Message {
	return slack.Message{
		Text: "Invoice #" + n.Invoice.Number + " was paid.",
		Attachments: []slack.Attachment{
			{
				Color: "good",
				Fields: []slack.Field{
					{Title: "Amount", Value: n.Invoice.Amount, Short: true},
					{Title: "Customer", Value: n.Invoice.CustomerName, Short: true},
				},
			},
		},
	}
}
```

Route to a channel or user by implementing `RouteNotificationFor` on
your notifiable model:

```go
func (u *User) RouteNotificationFor(channel string) any {
	if channel == "slack" {
		return "#billing" // or a user ID for a DM, e.g. "U0123ABC456"
	}
	return nil
}
```

A notification that doesn't implement `slack.Notification` still gets a
minimal default message (its Go type name) if `"slack"` is in `Via()` —
useful for quick alerts without writing a `ToSlack` method.

### On-demand notifications

```go
facades.Notification().
	Route("slack", "#alerts").
	Notify(&DeploymentFinished{})
```

## Testing

Inject a mock `client.Factory` directly — `slack.NewChannel` takes its
HTTP dependency via constructor, the same way every other built-in
notification channel does, so nothing here needs `facades.Http().Fake()`
or a real network call:

```go
http := mocksclient.NewFactory(t)
http.EXPECT().WithToken("xoxb-test").Return(http).Once()
http.EXPECT().WithHeader("Content-Type", "application/json; charset=utf-8").Return(http).Once()
http.EXPECT().Post(mock.Anything, mock.Anything).Return(resp, nil).Once()

ch := slack.NewChannel(http, "xoxb-test")
```

## Why not Incoming Webhooks?

Slack's older Incoming Webhooks mechanism binds one webhook URL to one
fixed channel, decided when the webhook is created — there's no way to
pick a different channel per notification. The Web API's `chat.postMessage`
takes the target channel as a request parameter instead, so a single bot
token can post anywhere it's been invited, and `RouteNotificationFor`
can vary per notifiable the same way the mail and database channels do.

One thing worth knowing if you're debugging delivery failures: Slack's
Web API returns HTTP 200 even when a request fails — the real success/
failure signal is the `"ok"` boolean in the JSON response body, not the
status code. This package checks that correctly; if you're ever
inspecting responses yourself, don't trust the status code alone.

## License

The Goravel Slack package is open-sourced software licensed under the
[MIT license](LICENSE).