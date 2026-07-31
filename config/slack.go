package config

import "github.com/goravel/framework/facades"

func init() {
	facades.Config().Add("slack", map[string]any{
		// token is the Slack bot token (xoxb-...). Create one at
		// https://api.slack.com/apps — needs the chat:write scope, and
		// the bot must be invited to any channel it should post in.
		"token": facades.Config().Env("SLACK_BOT_TOKEN", ""),
	})
}
